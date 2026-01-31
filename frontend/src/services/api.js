import axios from 'axios';

const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

const api = axios.create({
  baseURL: API_BASE,
});

export const uploadFiles = async (files, sessionId, signal) => {
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
    },
    signal // Support cancellation via AbortSignal
  });
  return response.data; // { filenames: ["..."] }
};

export const deleteSession = async (sessionId) => {
  // Use navigator.sendBeacon for final cleanup during unload if not using axios
  // But axios is fine for reset/redirect clicks
  try {
    const response = await api.delete(`/session?sessionId=${sessionId}`);
    return response.data;
  } catch (err) {
    console.warn("Session cleanup failed:", err);
  }
};

export const deleteFile = async (sessionId, filename) => {
  try {
    const response = await api.delete(`/file?sessionId=${sessionId}&filename=${encodeURIComponent(filename)}`);
    return response.data;
  } catch (err) {
    console.warn("File cleanup failed:", err);
  }
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

