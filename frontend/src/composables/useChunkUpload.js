import { computed, ref } from "vue";
import { completeMultipartUpload, initMultipartUpload, uploadMultipartPart } from "../api";

const DEFAULT_CHUNK_SIZE = 5 * 1024 * 1024;
const DEFAULT_CONCURRENCY = 3;
const MAX_RETRIES = 3;

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

  const isUploading = computed(() => status.value === "hashing" || status.value === "uploading" || status.value === "completing");

  async function upload(token, file) {
    reset();
    if (!file) throw new Error("文件不能为空");

    status.value = "hashing";
    totalBytes.value = file.size;
    const fileHash = await hashBlob(file);
    const totalChunks = Math.ceil(file.size / chunkSize);

    status.value = "initializing";
    const initRes = await initMultipartUpload(token, {
      fileName: file.name,
      contentType: file.type || "application/octet-stream",
      size: file.size,
      fileHash,
      chunkSize,
      totalChunks,
    });

    if (initRes.completed && initRes.file) {
      progress.value = 100;
      uploadedBytes.value = file.size;
      status.value = "completed";
      return initRes.file;
    }

    uploadId.value = initRes.uploadId;
    const uploadedSet = new Set(initRes.uploadedParts || []);
    uploadedBytes.value = uploadedUploadedBytes(file, uploadedSet, totalChunks, chunkSize);
    updateProgress();

    status.value = "uploading";
    const queue = [];
    for (let partNumber = 1; partNumber <= totalChunks; partNumber += 1) {
      if (!uploadedSet.has(partNumber)) queue.push(partNumber);
    }

    let cursor = 0;
    async function worker() {
      while (cursor < queue.length) {
        if (canceled.value) throw new Error("上传已取消");
        while (paused.value) await sleep(200);
        const partNumber = queue[cursor];
        cursor += 1;
        await uploadPartWithRetry(token, file, uploadId.value, partNumber, totalChunks);
      }
    }

    await Promise.all(Array.from({ length: Math.min(concurrency, queue.length) }, () => worker()));

    status.value = "completing";
    const fileRes = await completeMultipartUpload(token, uploadId.value);
    progress.value = 100;
    uploadedBytes.value = file.size;
    status.value = "completed";
    return fileRes;
  }

  function pause() {
    if (status.value === "uploading") {
      paused.value = true;
      status.value = "paused";
    }
  }

  function resume() {
    if (status.value === "paused") {
      paused.value = false;
      status.value = "uploading";
    }
  }

  function cancel() {
    canceled.value = true;
    status.value = "canceled";
  }

  function reset() {
    status.value = "idle";
    progress.value = 0;
    uploadedBytes.value = 0;
    totalBytes.value = 0;
    error.value = "";
    uploadId.value = "";
    paused.value = false;
    canceled.value = false;
  }

  async function uploadPartWithRetry(token, file, currentUploadId, partNumber, totalChunks) {
    const start = (partNumber - 1) * chunkSize;
    const end = Math.min(file.size, start + chunkSize);
    const chunk = file.slice(start, end);
    const chunkHash = await hashBlob(chunk);

    for (let attempt = 1; attempt <= MAX_RETRIES; attempt += 1) {
      try {
        await uploadMultipartPart(token, currentUploadId, partNumber, chunk, chunkHash);
        uploadedBytes.value += chunk.size;
        updateProgress();
        return;
      } catch (err) {
        if (attempt === MAX_RETRIES) {
          error.value = err.message;
          throw err;
        }
        await sleep(500 * attempt);
      }
    }
  }

  function updateProgress() {
    if (!totalBytes.value) {
      progress.value = 0;
      return;
    }
    progress.value = Math.min(100, Math.floor((uploadedBytes.value / totalBytes.value) * 100));
  }

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

function uploadedUploadedBytes(file, uploadedSet, totalChunks, chunkSize) {
  let bytes = 0;
  for (const partNumber of uploadedSet) {
    const start = (partNumber - 1) * chunkSize;
    const end = partNumber === totalChunks ? file.size : Math.min(file.size, start + chunkSize);
    bytes += Math.max(0, end - start);
  }
  return bytes;
}

async function hashBlob(blob) {
  const data = await blob.arrayBuffer();
  const digest = await crypto.subtle.digest("SHA-256", data);
  return Array.from(new Uint8Array(digest))
    .map((item) => item.toString(16).padStart(2, "0"))
    .join("");
}

function sleep(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
