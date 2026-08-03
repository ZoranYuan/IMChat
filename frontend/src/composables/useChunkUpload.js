import { computed, ref } from "vue";
import {
  completeDirectUpload,
  completeMultipartUpload,
  initDirectUpload,
  initMultipartUpload,
  presignMultipartParts,
  uploadDirectObjectToStorage,
  uploadMultipartPartToStorage,
} from "../api.js";

const DEFAULT_CHUNK_SIZE = 5 * 1024 * 1024;
const DEFAULT_CONCURRENCY = 3;
const MULTIPART_THRESHOLD = 8 * 1024 * 1024;
const MAX_RETRIES = 3;
const MAX_COMPLETE_REPAIR_ROUNDS = 2;
const FINGERPRINT_SAMPLE_SIZE = 2 * 1024 * 1024;

export function useChunkUpload(options = {}) {
  const chunkSize = options.chunkSize || DEFAULT_CHUNK_SIZE;
  const concurrency = options.concurrency || DEFAULT_CONCURRENCY;
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
        return await completeMultipartUpload(currentUploadId);
      } catch (completeError) {
        const repairParts = incompletePartNumbers(completeError);
        if (!repairParts.length || round === MAX_COMPLETE_REPAIR_ROUNDS) throw completeError;
        status.value = "uploading";
        await repairIncompleteParts(file, currentUploadId, repairParts, completedParts);
      }
    }
    throw new Error("合并文件失败");
  };

  const uploadMultipart = async (file) => {
    status.value = "hashing";
    const fileHash = await createFileFingerprint(file);
    const totalChunks = Math.ceil(file.size / chunkSize);
    status.value = "initializing";
    const initialized = await initMultipartUpload({
      fileName: file.name,
      contentType: file.type || "application/octet-stream",
      size: file.size,
      fileHash,
      chunkSize,
      totalChunks,
    });
    if (initialized.status === "completed") return { fileId: initialized.fileId };

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
  const uploadDirect = async (file) => {
    status.value = "hashing";
    const fileHash = await createActualSHA256(file);
    status.value = "initializing";
    const initialized = await initDirectUpload({
      fileName: file.name,
      contentType: file.type || "application/octet-stream",
      size: file.size,
      fileHash,
    });
    if (initialized.status === "completed") return { fileId: initialized.fileId };

    uploadId.value = initialized.uploadId;
    status.value = "uploading";
    await uploadDirectObjectToStorage(initialized.url, file, (event) => {
      if (event.total) {
        uploadedBytes.value = Math.min(file.size, event.loaded);
        updateProgress();
      }
    });
    status.value = "completing";
    return completeDirectUpload(initialized.uploadId);
  };

  const upload = async (file) => {
    reset();
    if (!file || file.size <= 0) throw new Error("文件不能为空");
    totalBytes.value = file.size;
    try {
      let result;
      if (file.size < MULTIPART_THRESHOLD) {
        result = await uploadDirect(file);
      } else {
        result = await uploadMultipart(file);
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

async function createFileFingerprint(file) {
  const sampleSize = FINGERPRINT_SAMPLE_SIZE;
  const samples = file.size <= sampleSize * 3
    ? [file]
    : [
      file.slice(0, sampleSize),
      file.slice(Math.floor(file.size / 2 - sampleSize / 2), Math.floor(file.size / 2 + sampleSize / 2)),
      file.slice(file.size - sampleSize, file.size),
    ];
  const buffers = await Promise.all(samples.map((sample) => sample.arrayBuffer()));
  const metadata = new TextEncoder().encode([file.name, file.size, file.type, file.lastModified].join("|"));
  const size = buffers.reduce((total, buffer) => total + buffer.byteLength, metadata.byteLength);
  const payload = new Uint8Array(size);
  payload.set(metadata);
  let offset = metadata.byteLength;
  buffers.forEach((buffer) => {
    payload.set(new Uint8Array(buffer), offset);
    offset += buffer.byteLength;
  });
  return digest(payload);
}

async function createActualSHA256(file) {
  return digest(await file.arrayBuffer());
}

const digest = async (buffer) => {
  const result = await crypto.subtle.digest("SHA-256", buffer);
  return Array.from(new Uint8Array(result), (value) => value.toString(16).padStart(2, "0")).join("");
};
const sleep = (duration) => new Promise((resolve) => window.setTimeout(resolve, duration));

function incompletePartNumbers(error) {
  if (error?.status !== 409) return [];
  const data = error.data || {};
  const numbers = [...(data.missingParts || []), ...(data.invalidParts || [])]
    .map((partNumber) => Number(partNumber))
    .filter((partNumber) => Number.isInteger(partNumber) && partNumber > 0);
  return [...new Set(numbers)].sort((a, b) => a - b);
}
