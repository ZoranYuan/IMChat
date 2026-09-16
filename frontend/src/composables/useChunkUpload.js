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
  const error = ref("");
  const uploadId = ref("");
  const paused = ref(false);
  const canceled = ref(false);
  // 收集所有的上传任务，当取消时，取消上传中的任务
  const activeRequests = new Set();
  const isUploading = computed(() =>
    ["hashing", "initializing", "uploading", "completing"].includes(status.value),
  );

  console.log("开始上传文件")

  const reset = () => {
    activeRequests.forEach((controller) => controller.abort());
    activeRequests.clear();
    status.value = "idle";
    progress.value = 0;
    error.value = "";
    uploadId.value = "";
    paused.value = false;
    canceled.value = false;
    uploader.releasePause?.();
    uploader.releasePause = null;
    resumePromise = null;
  };

  const updateMultipartProgress = (completedParts, totalChunks) => {
    console.log("更新上传进度")
    progress.value = totalChunks > 0
      ? Math.min(100, Math.floor((completedParts.size / totalChunks) * 100))
      : 0;
  };

  let resumePromise = null;

  const waitUntilResumed = () => {
    if (!paused.value) return Promise.resolve();

    if (!resumePromise) {
      resumePromise = new Promise((resolve) => {
        uploader.releasePause = () => {
          paused.value = false;
          resumePromise = null;
          uploader.releasePause = null;
          resolve();
        };
      });
    }

    return resumePromise;
  };

  const uploadPartWithRetry = async (
    file,
    currentUploadId,
    partNumber,
    partURLs,
    completedParts,
    totalChunks,
  ) => {
    const start = (partNumber - 1) * chunkSize;
    const chunk = file.slice(start, Math.min(file.size, start + chunkSize));
    for (let attempt = 1; attempt <= MAX_RETRIES; attempt += 1) {
      try {
        const partURL = partURLs.get(partNumber);
        if (!partURL) throw new Error("缺少分片上传地址");

        const controller = new AbortController();
        activeRequests.add(controller);
        try {
          await uploadMultipartPartToStorage(
            partURL,
            chunk,
            controller.signal,
          );
        } finally {
          activeRequests.delete(controller);
        }

        completedParts.add(partNumber);
        updateMultipartProgress(completedParts, totalChunks);
        return;
      } catch (uploadError) {
        if (
          canceled.value
          || uploadError?.name === "CanceledError"
          || uploadError?.name === "AbortError"
          || uploadError?.code === "ERR_CANCELED"
        ) throw uploadError;

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

  const repairIncompleteParts = async (
    file,
    currentUploadId,
    partNumbers,
    completedParts,
    totalChunks,
  ) => {
    if (!partNumbers.length) return;
    for (const partNumber of partNumbers) {
      completedParts.delete(partNumber);
    }
    updateMultipartProgress(completedParts, totalChunks);
    const partURLs = await presignParts(currentUploadId, partNumbers);
    let cursor = 0;
    const worker = async () => {
      while (cursor < partNumbers.length) {
        if (canceled.value) throw new Error("上传已取消");
        await waitUntilResumed();
        if (canceled.value) throw new Error("上传已取消");
        const partNumber = partNumbers[cursor];
        cursor += 1;
        await uploadPartWithRetry(
          file,
          currentUploadId,
          partNumber,
          partURLs,
          completedParts,
          totalChunks,
        );
      }
    };
    await Promise.all(Array.from({ length: Math.min(concurrency, partNumbers.length) }, worker));
  };

  const completeWithRepair = async (
    file,
    currentUploadId,
    completedParts,
    totalChunks,
  ) => {
    for (let round = 0; round <= MAX_COMPLETE_REPAIR_ROUNDS; round += 1) {
      try {
        if (canceled.value) throw new Error("上传已取消");
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
        await repairIncompleteParts(
          file,
          currentUploadId,
          repairParts,
          completedParts,
          totalChunks,
        );
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
    updateMultipartProgress(completedParts, totalChunks);

    const queue = Array.from({ length: totalChunks }, (_, index) => index + 1)
      .filter((partNumber) => !completedParts.has(partNumber));
    const partURLs = await presignParts(initialized.uploadId, queue);
    let cursor = 0;
    status.value = "uploading";

    const worker = async () => {
      while (cursor < queue.length) {
        if (canceled.value) throw new Error("上传已取消");
        await waitUntilResumed();
        if (canceled.value) throw new Error("上传已取消");
        const partNumber = queue[cursor];
        console.log("开始上传，当前 cursor 为：", cursor)
        cursor += 1;
        await uploadPartWithRetry(
          file,
          initialized.uploadId,
          partNumber,
          partURLs,
          completedParts,
          totalChunks,
        );
      }
    };
    await Promise.all(Array.from({ length: Math.min(concurrency, queue.length) }, worker));
    progress.value = cursor;
    if (canceled.value) throw new Error("上传已取消");
    return completeWithRepair(
      file,
      initialized.uploadId,
      completedParts,
      totalChunks,
    );
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
    const controller = new AbortController();
    activeRequests.add(controller);
    console.log("开始上传")
    try {
      await uploadDirectObjectToStorage(initialized.url, file, controller.signal);
      console.log("上传完成")
      progress.value = 100;
    } finally {
      console.log("上传失败")
      activeRequests.delete(controller);
    }
    if (canceled.value) throw new Error("上传已取消");
    status.value = "completing";
    console.log("上传完成，准备调用 complete 接口")
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
      status.value = "hashing";
      const fileHash = await createActualSHA256(file);
      if (canceled.value) throw new Error("上传已取消");
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
      if (canceled.value) throw new Error("上传已取消");

      if (initialized.status === "completed" && initialized.fileId) {
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
      if (uploader.releasePause) {
        uploader.releasePause();
      } else {
        paused.value = false;
        resumePromise = null;
      }
      status.value = "uploading";
    }
  };

  const cancel = () => {
    canceled.value = true;
    paused.value = false;
    uploader.releasePause?.();
    uploader.releasePause = null;
    resumePromise = null;
    activeRequests.forEach((controller) => controller.abort());
    activeRequests.clear();
    status.value = "canceled";
  };

  const uploader = {
    status,
    progress,
    error,
    uploadId,
    isUploading,
    upload,
    pause,
    resume,
    cancel,
    reset,
    releasePause: null,
  };

  return uploader;
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
