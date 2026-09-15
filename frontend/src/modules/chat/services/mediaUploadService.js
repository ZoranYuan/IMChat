import { MessageType } from "../../../constants/message.js";

const assertFile = (file) => {
  if (!file || typeof file.size !== "number" || file.size <= 0) {
    throw new Error("文件不能为空");
  }
};

class BaseMediaUploader {
  constructor(binaryUploader) {
    this.binaryUploader = binaryUploader;
  }

  async uploadBinary(file) {
    assertFile(file);
    const uploaded = await this.binaryUploader.upload(file);
    if (!uploaded?.fileId) {
      throw new Error("上传成功但未返回文件标识。");
    }

    return {
      fileId: uploaded.fileId,
      status: uploaded.status || "completed",
    };
  }
}

export class ImageUploader extends BaseMediaUploader {
  async upload(file) {
    assertFile(file);
    if (file.type && !file.type.startsWith("image/")) {
      throw new Error("请选择图片文件。");
    }

    return {
      ...(await this.uploadBinary(file)),
      cType: MessageType.IMAGE,
    };
  }
}

export class VideoUploader extends BaseMediaUploader {
  async upload(file) {
    assertFile(file);
    if (file.type && !file.type.startsWith("video/")) {
      throw new Error("请选择视频文件。");
    }

    return {
      ...(await this.uploadBinary(file)),
      cType: MessageType.VIDEO,
    };
  }
}

export class FileUploader extends BaseMediaUploader {
  async upload(file) {
    return {
      ...(await this.uploadBinary(file)),
      cType: MessageType.FILE,
    };
  }
}

export function createMediaUploadService(binaryUploader) {
  const uploaders = new Map([
    [MessageType.IMAGE, new ImageUploader(binaryUploader)],
    [MessageType.VIDEO, new VideoUploader(binaryUploader)],
    [MessageType.FILE, new FileUploader(binaryUploader)],
  ]);

  return {
    upload(file, cType) {
      const uploader = uploaders.get(Number(cType));
      if (!uploader) throw new Error("不支持的媒体类型。");
      return uploader.upload(file);
    },
  };
}
