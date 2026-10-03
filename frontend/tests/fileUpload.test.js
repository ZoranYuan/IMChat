import assert from "node:assert/strict";
import { createHash, webcrypto } from "node:crypto";
import { afterEach, test } from "node:test";
import axios from "axios";
import http, { ApiError } from "../src/api.js";
import { useFileUpload } from "../src/composables/useFileUpload.js";
import { UploadStatus } from "../src/constants/upload.js";

if (!globalThis.crypto) globalThis.crypto = webcrypto;

const originalAPIAdapter = http.defaults.adapter;
const originalStorageAdapter = axios.defaults.adapter;

afterEach(() => {
  http.defaults.adapter = originalAPIAdapter;
  axios.defaults.adapter = originalStorageAdapter;
});

const response = (config, data, status = 200) => ({
  config, data: { code: status, message: "success", data },
  status, statusText: "OK", headers: {},
});

const sampleFile = () => Object.assign(
  new Blob(["abcdef"], { type: "text/plain" }),
  { name: "sample.txt" },
);

test("秒传通过统一入口返回，并完成状态和进度", async () => {
  let calls = 0;
  http.defaults.adapter = async (config) => {
    calls += 1;
    assert.equal(config.url, "/files/uploads/init");
    const request = JSON.parse(config.data);
    assert.equal(request.fileHash, createHash("sha256").update("abcdef").digest("hex"));
    return response(config, { status: "completed", fileId: "file-1" });
  };
  const uploader = useFileUpload();
  assert.equal((await uploader.upload(sampleFile())).fileId, "file-1");
  assert.equal(calls, 1);
  assert.equal(uploader.status.value, UploadStatus.COMPLETED);
  assert.equal(uploader.progress.value, 100);
  assert.equal(uploader.isUploading.value, false);
});

test("直传使用预签名地址，随后通过统一完成接口校验", async () => {
  const calls = [];
  http.defaults.adapter = async (config) => {
    calls.push(config.url);
    if (config.url.endsWith("/init")) {
      return response(config, { status: "uploading", uploadMode: "direct", uploadId: "upload-1", url: "http://storage.example/direct" });
    }
    assert.equal(config.url, "/files/uploads/upload-1/complete");
    return response(config, { status: "completed", fileId: "file-1" });
  };
  axios.defaults.adapter = async (config) => {
    calls.push(config.url);
    assert.equal(config.method, "put");
    assert.equal(config.data.size, 6);
    return { config, data: "", status: 200, statusText: "OK", headers: {} };
  };
  const uploader = useFileUpload();
  await uploader.upload(sampleFile());
  assert.deepEqual(calls, ["/files/uploads/init", "http://storage.example/direct", "/files/uploads/upload-1/complete"]);
  assert.equal(uploader.status.value, UploadStatus.COMPLETED);
  assert.equal(uploader.progress.value, 100);
});

test("分片续传只上传缺片，完成时进度保持百分比", async () => {
  const uploadedParts = [];
  const uploader = useFileUpload({ chunkSize: 2 });
  http.defaults.adapter = async (config) => {
    if (config.url.endsWith("/init")) {
      return response(config, { status: "uploading", uploadMode: "multipart", uploadId: "upload-2", uploadedParts: [1] });
    }
    if (config.url.endsWith("/parts/presign")) {
      assert.equal(config.url, "/files/uploads/upload-2/parts/presign");
      const { partNumbers } = JSON.parse(config.data);
      assert.deepEqual(partNumbers, [2, 3]);
      return response(config, partNumbers.map((partNumber) => ({ partNumber, url: `http://storage.example/${partNumber}` })));
    }
    assert.equal(config.url, "/files/uploads/upload-2/complete");
    assert.equal(uploader.progress.value, 100);
    return response(config, { status: "completed", fileId: "file-2" });
  };
  axios.defaults.adapter = async (config) => {
    uploadedParts.push(Number(config.url.split("/").at(-1)));
    assert.equal(config.data.size, 2);
    return { config, data: "", status: 200, statusText: "OK", headers: {} };
  };
  await uploader.upload(sampleFile());
  assert.deepEqual(uploadedParts.sort(), [2, 3]);
  assert.equal(uploader.status.value, UploadStatus.COMPLETED);
  assert.equal(uploader.progress.value, 100);
});

test("API 只接受当前响应封装，并保留失败响应数据", async () => {
  for (const data of [[], { code: 0, data: [] }]) {
    http.defaults.adapter = async (config) => ({ ...response(config, null), data });
    await assert.rejects(http.get("/conversations"), ApiError);
  }
  const incomplete = { status: "uploading", missingParts: [2], invalidParts: [] };
  http.defaults.adapter = async (config) => response(config, incomplete, 409);
  await assert.rejects(http.post("/files/uploads/upload-2/complete"), (error) => {
    assert.equal(error.status, 409);
    assert.deepEqual(error.data, incomplete);
    return true;
  });
});
