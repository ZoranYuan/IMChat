import { computed, ref } from "vue";
import {
  completeMultipartUpload,
  initMultipartUpload,
  uploadFile,
  uploadMultipartPart,
} from "../api.js";

const DEFAULT_CHUNK_SIZE = 5 * 1024 * 1024;
const DEFAULT_CONCURRENCY = 3;
const MULTIPART_THRESHOLD = 8 * 1024 * 1024;
const MAX_RETRIES = 3;
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

  const uploadPartWithRetry = async (token, file, currentUploadId, partNumber) => {
    const start = (partNumber - 1) * chunkSize;
    const chunk = file.slice(start, Math.min(file.size, start + chunkSize));
    const chunkHash = await hashBlob(chunk);
    for (let attempt = 1; attempt <= MAX_RETRIES; attempt += 1) {
      try {
        await uploadMultipartPart(token, currentUploadId, partNumber, chunk, chunkHash);
        uploadedBytes.value += chunk.size;
        updateProgress();
        return;
      } catch (uploadError) {
        if (attempt === MAX_RETRIES) throw uploadError;
        await sleep(500 * attempt);
      }
    }
  };

  const uploadMultipart = async (token, file) => {
    status.value = "hashing";
    const fileHash = await createFileFingerprint(file);
    const totalChunks = Math.ceil(file.size / chunkSize);
    status.value = "initializing";
    const initialized = await initMultipartUpload(token, {
      fileName: file.name,
      contentType: file.type || "application/octet-stream",
      size: file.size,
      fileHash,
      chunkSize,
      totalChunks,
    });
    if (initialized.completed && initialized.file) return initialized.file;

    uploadId.value = initialized.uploadId;
    const uploadedParts = new Set(initialized.uploadedParts || []);
    for (const partNumber of uploadedParts) {
      const start = (partNumber - 1) * chunkSize;
      uploadedBytes.value += Math.max(0, Math.min(file.size, start + chunkSize) - start);
    }
    updateProgress();

    const queue = Array.from({ length: totalChunks }, (_, index) => index + 1)
      .filter((partNumber) => !uploadedParts.has(partNumber));
    let cursor = 0;
    status.value = "uploading";

    const worker = async () => {
      while (cursor < queue.length) {
        if (canceled.value) throw new Error("上传已取消");
        while (paused.value) await sleep(150);
        if (canceled.value) throw new Error("上传已取消");
        const partNumber = queue[cursor];
        cursor += 1;
        await uploadPartWithRetry(token, file, initialized.uploadId, partNumber);
      }
    };
    await Promise.all(Array.from({ length: Math.min(concurrency, queue.length) }, worker));
    status.value = "completing";
    return completeMultipartUpload(token, initialized.uploadId);
  };

  const upload = async (token, file) => {
    reset();
    if (!file || file.size <= 0) throw new Error("文件不能为空");
    totalBytes.value = file.size;
    try {
      let result;
      if (file.size < MULTIPART_THRESHOLD) {
        status.value = "uploading";
        result = await uploadFile(token, file);
      } else {
        result = await uploadMultipart(token, file);
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

const hashBlob = async (blob) => digest(await blob.arrayBuffer());
const digest = async (buffer) => {
  const result = await crypto.subtle.digest("SHA-256", buffer);
  return Array.from(new Uint8Array(result), (value) => value.toString(16).padStart(2, "0")).join("");
};
const sleep = (duration) => new Promise((resolve) => window.setTimeout(resolve, duration));
