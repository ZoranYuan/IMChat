import { computed, ref } from "vue";
import {
  completeUpload,
  initUpload,
  presignMultipartParts,
  uploadDirectObjectToStorage,
  uploadMultipartPartToStorage,
} from "../api.js";

const DEFAULT_CHUNK_SIZE = 5 * 1024 * 1024;
const DEFAULT_CONCURRENCY = 3;
const DEFAULT_MAX_FILE_SIZE = 1024 * 1024 * 1024;
const DEFAULT_MAX_HASH_FILE_SIZE = 512 * 1024 * 1024;
const HASH_MEMORY_MULTIPLIER = 2;
const DEVICE_HASH_MEMORY_RATIO = 0.25;
const MAX_RETRIES = 3;
const MAX_COMPLETE_REPAIR_ROUNDS = 2;

export function useChunkUpload(options = {}) {
  const chunkSize = options.chunkSize || DEFAULT_CHUNK_SIZE;
  const concurrency = options.concurrency || DEFAULT_CONCURRENCY;
  const maxFileSize = options.maxFileSize || DEFAULT_MAX_FILE_SIZE;
  const maxHashFileSize = options.maxHashFileSize || DEFAULT_MAX_HASH_FILE_SIZE;
  const status = ref("idle");
  const progress = ref(0);
  const uploadedBytes = ref(0);
  const totalBytes = ref(0);
  const error = ref("");
  const uploadId = ref("");
  const paused = ref(false);
  const canceled = ref(false);

  const isUploading = computed(() =>
    ["hashing", "initializing", "uploading", "completing"].includes(status.value),
  );

  const updateProgress = () => {
    progress.value = totalBytes.value
      ? Math.min(100, Math.floor((uploadedBytes.value / totalBytes.value) * 100))
      : 0;
  };

  const reset = () => {
    status.value = "idle";
    progress.value = 0;
    uploadedBytes.value = 0;
    totalBytes.value = 0;
    error.value = "";
    uploadId.value = "";
    paused.value = false;
    canceled.value = false;
  };

  const uploadPartWithRetry = async (file, currentUploadId, partNumber, partURLs, completedParts) => {
    const start = (partNumber - 1) * chunkSize;
    const chunk = file.slice(start, Math.min(file.size, start + chunkSize));
    for (let attempt = 1; attempt <= MAX_RETRIES; attempt += 1) {
      try {
        const partURL = partURLs.get(partNumber);
        if (!partURL) throw new Error("缺少分片上传地址");
        await uploadMultipartPartToStorage(partURL, chunk);
        if (!completedParts.has(partNumber)) {
          completedParts.add(partNumber);
          uploadedBytes.value += chunk.size;
          updateProgress();
        }
        return;
      } catch (uploadError) {
        if (attempt === MAX_RETRIES) throw uploadError;
        try {
          const refreshed = await presignMultipartParts(currentUploadId, [partNumber]);
          if (refreshed[0]) partURLs.set(partNumber, refreshed[0].url);
        } catch {
          // 保留原上传错误，下一次循环仍会按重试策略处理。
        }
        await sleep(500 * attempt);
      }
    }
  };

  const presignParts = async (currentUploadId, partNumbers) => {
    const partURLs = new Map();
    for (let offset = 0; offset < partNumbers.length; offset += 100) {
      const batch = await presignMultipartParts(currentUploadId, partNumbers.slice(offset, offset + 100));
      batch.forEach((part) => partURLs.set(part.partNumber, part.url));
    }
    return partURLs;
  };

  const repairIncompleteParts = async (file, currentUploadId, partNumbers, completedParts) => {
    if (!partNumbers.length) return;
    const partURLs = await presignParts(currentUploadId, partNumbers);
    let cursor = 0;
    const worker = async () => {
      while (cursor < partNumbers.length) {
        if (canceled.value) throw new Error("上传已取消");
        while (paused.value) await sleep(150);
        if (canceled.value) throw new Error("上传已取消");
        const partNumber = partNumbers[cursor];
        cursor += 1;
        await uploadPartWithRetry(file, currentUploadId, partNumber, partURLs, completedParts);
      }
    };
    await Promise.all(Array.from({ length: Math.min(concurrency, partNumbers.length) }, worker));
  };

  const completeWithRepair = async (file, currentUploadId, completedParts) => {
    for (let round = 0; round <= MAX_COMPLETE_REPAIR_ROUNDS; round += 1) {
      try {
        status.value = "completing";
        const result = await completeUpload(currentUploadId);
        if (result?.status !== "completed" || !result.fileId) {
          throw new Error("合并成功但未返回文件标识。");
        }
        return result;
      } catch (completeError) {
        const repairParts = incompletePartNumbers(completeError);
        if (!repairParts.length || round === MAX_COMPLETE_REPAIR_ROUNDS) throw completeError;
        status.value = "uploading";
        await repairIncompleteParts(file, currentUploadId, repairParts, completedParts);
      }
    }
    throw new Error("合并文件失败");
  };

  const uploadMultipart = async (file, initialized) => {
    const totalChunks = Math.ceil(file.size / chunkSize);
    if (initialized.status === "completed" && initialized.fileId) {
      return { fileId: initialized.fileId, status: initialized.status };
    }
    if (initialized.status === "completed") {
      throw new Error("秒传响应缺少文件标识。");
    }

    uploadId.value = initialized.uploadId;
    const completedParts = new Set(initialized.uploadedParts || []);
    for (const partNumber of completedParts) {
      const start = (partNumber - 1) * chunkSize;
      uploadedBytes.value += Math.max(0, Math.min(file.size, start + chunkSize) - start);
    }
    updateProgress();

    const queue = Array.from({ length: totalChunks }, (_, index) => index + 1)
      .filter((partNumber) => !completedParts.has(partNumber));
    const partURLs = await presignParts(initialized.uploadId, queue);
    let cursor = 0;
    status.value = "uploading";

    const worker = async () => {
      while (cursor < queue.length) {
        if (canceled.value) throw new Error("上传已取消");
        while (paused.value) await sleep(150);
        if (canceled.value) throw new Error("上传已取消");
        const partNumber = queue[cursor];
        cursor += 1;
        await uploadPartWithRetry(file, initialized.uploadId, partNumber, partURLs, completedParts);
      }
    };
    await Promise.all(Array.from({ length: Math.min(concurrency, queue.length) }, worker));
    return completeWithRepair(file, initialized.uploadId, completedParts);
  };

  // 小文件前端直接传递
  const uploadDirect = async (file, initialized) => {
    // 只有秒传命中时才会返回 fileId；普通直传初始化不会返回 fileId。
    if (initialized.status === "completed" && initialized.fileId) {
      return { fileId: initialized.fileId, status: initialized.status };
    }
    if (initialized.status === "completed") {
      throw new Error("秒传响应缺少文件标识。");
    }

    // 之前没传递过，开始根据后端返回的 url 传递数
    uploadId.value = initialized.uploadId;
    status.value = "uploading";
    await uploadDirectObjectToStorage(initialized.url, file, (event) => {
      if (event.total) {
        uploadedBytes.value = Math.min(file.size, event.loaded);
        updateProgress();
      }
    });
    status.value = "completing";
    const result = await completeUpload(initialized.uploadId);
    if (result?.status !== "completed" || !result.fileId) {
      throw new Error("上传完成但未返回文件标识。");
    }
    return result;
  };

  const upload = async (file) => {
    reset();
    try {
      if (!file || file.size <= 0) throw new Error("文件不能为空");
      assertFileCanBeHashed(file, maxFileSize, maxHashFileSize);
      totalBytes.value = file.size;

      status.value = "hashing";
      const fileHash = await createActualSHA256(file);
      const totalChunks = Math.ceil(file.size / chunkSize);
      status.value = "initializing";
      const initialized = await initUpload({
        fileName: file.name,
        contentType: file.type || "application/octet-stream",
        size: file.size,
        fileHash,
        chunkSize,
        totalChunks,
      });

      if (initialized.status === "completed" && initialized.fileId) {
        uploadedBytes.value = file.size;
        return { fileId: initialized.fileId, status: initialized.status };
      }
      if (initialized.status === "completed") {
        throw new Error("秒传响应缺少文件标识。");
      }

      let result;
      if (initialized.uploadMode === "direct") {
        result = await uploadDirect(file, initialized);
      } else if (initialized.uploadMode === "multipart") {
        result = await uploadMultipart(file, initialized);
      } else {
        throw new Error("服务端返回了未知的上传模式。");
      }
      if (canceled.value) throw new Error("上传已取消");
      uploadedBytes.value = file.size;
      progress.value = 100;
      status.value = "completed";
      return result;
    } catch (uploadError) {
      error.value = uploadError.message;
      if (!canceled.value) status.value = "failed";
      throw uploadError;
    }
  };

  const pause = () => {
    if (status.value === "uploading") {
      paused.value = true;
      status.value = "paused";
    }
  };
  const resume = () => {
    if (status.value === "paused") {
      paused.value = false;
      status.value = "uploading";
    }
  };
  const cancel = () => {
    canceled.value = true;
    paused.value = false;
    status.value = "canceled";
  };

  return {
    status,
    progress,
    uploadedBytes,
    totalBytes,
    error,
    uploadId,
    isUploading,
    upload,
    pause,
    resume,
    cancel,
    reset,
  };
}

async function createActualSHA256(file) {
  return digest(await file.arrayBuffer());
}

function assertFileCanBeHashed(file, maxFileSize, maxHashFileSize) {
  if (file.size > maxFileSize) {
    throw new Error("文件大小超过允许上传的上限。");
  }

  if (file.size > maxHashFileSize) {
    throw new Error("当前设备不适合在浏览器中计算该文件的完整 Hash。");
  }

  const deviceMemoryGB = Number(globalThis.navigator?.deviceMemory);
  if (!Number.isFinite(deviceMemoryGB) || deviceMemoryGB <= 0) return;

  const estimatedHashMemory = file.size * HASH_MEMORY_MULTIPLIER;
  const deviceHashMemoryLimit = deviceMemoryGB
    * 1024 * 1024 * 1024
    * DEVICE_HASH_MEMORY_RATIO;
  if (estimatedHashMemory > deviceHashMemoryLimit) {
    throw new Error("当前设备可用内存不足，无法安全计算该文件的完整 Hash。");
  }
}

const digest = async (buffer) => {
  const result = await crypto.subtle.digest("SHA-256", buffer);
  return Array.from(new Uint8Array(result), (value) => value.toString(16).padStart(2, "0")).join("");
};
const sleep = (duration) => new Promise((resolve) => window.setTimeout(resolve, duration));

function incompletePartNumbers(error) {
  if (error?.status !== 409 || error.data?.status !== "uploading") return [];
  const data = error.data || {};
  const numbers = [...(data.missingParts || []), ...(data.invalidParts || [])]
    .map((partNumber) => Number(partNumber))
    .filter((partNumber) => Number.isInteger(partNumber) && partNumber > 0);
  return [...new Set(numbers)].sort((a, b) => a - b);
}
