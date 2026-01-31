import axios from 'axios';

const API_BASE = 'http://localhost:8080';

const api = axios.create({
  baseURL: API_BASE,
});

export const uploadFiles = async (files, sessionId) => {
  const formData = new FormData();

  // files is Array of File objects
  if (files.length) {
    for (let i = 0; i < files.length; i++) {
      const f = files[i];
      // Use relativePath if available (from folder scan), else name
      const filename = f.relativePath || f.name;
      // Append explicit path to ensure backend gets it even if browser strips it from filename
      formData.append('paths', filename);
      // Append with filename as third argument to preserve path
      formData.append('files', f, filename);
    }
  }

  const response = await api.post('/upload', formData, {
    headers: {
      'X-Session-ID': sessionId
    }
  });
  return response.data; // { filenames: ["..."] }
};

export const compressFiles = async (filenames, sessionId) => {
  const response = await api.post('/compress', { filenames, sessionId });
  return response.data; // { downloadUrl: "...", message: "..." }
};

export const extractFiles = async (filenames, sessionId, password) => {
  const response = await api.post('/extract', { filenames, sessionId, password });
  return response.data; // { files: ["..."], message: "..." }
};

export const getDownloadUrl = (path) => {
  if (path.startsWith('http')) return path;
  return `${API_BASE}${path}`;
};
