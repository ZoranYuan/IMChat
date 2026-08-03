export const UploadStatus = Object.freeze({
  IDLE: "idle",
  HASHING: "hashing",
  INITIALIZING: "initializing",
  UPLOADING: "uploading",
  COMPLETING: "completing",
  PAUSED: "paused",
  COMPLETED: "completed",
  FAILED: "failed",
  CANCELED: "canceled",
});

export const ActiveUploadStatuses = Object.freeze([
  UploadStatus.HASHING,
  UploadStatus.INITIALIZING,
  UploadStatus.UPLOADING,
  UploadStatus.COMPLETING,
]);
