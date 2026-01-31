import React, { useState, useRef, useEffect } from 'react';
import { UploadCloud, FileArchive, Download, RefreshCw, X, CheckCircle, AlertCircle, FileText, Plus, Folder } from 'lucide-react';
import { uploadFiles, compressFiles, extractFiles, getDownloadUrl, deleteSession, deleteFile } from './services/api';
import { scanFiles } from './utils/fileScanner';

// Simple UUID generator
const generateUUID = () => {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
    var r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
};

function App() {
  const [files, setFiles] = useState([]);
  const [backendFilenames, setBackendFilenames] = useState([]); // relative paths
  const [status, setStatus] = useState('idle');
  const [mode, setMode] = useState(null);
  const [result, setResult] = useState(null);
  const [errorMsg, setErrorMsg] = useState('');
  const [sessionId, setSessionId] = useState('');
  const [showPasswordInput, setShowPasswordInput] = useState(false);
  const [password, setPassword] = useState('');

  const fileInputRef = useRef(null);
  const folderInputRef = useRef(null);
  const uploadAbortControllerRef = useRef(null);
  const sessionIdRef = useRef('');

  useEffect(() => {
    const newSessionId = generateUUID();
    setSessionId(newSessionId);
    sessionIdRef.current = newSessionId;

    const cleanup = () => {
      // Use the ref to get the absolute latest sessionId
      const currentId = sessionIdRef.current;
      if (currentId) {
        const url = `${import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'}/session?sessionId=${currentId}`;
        navigator.sendBeacon(url);
      }
    };

    window.addEventListener('beforeunload', cleanup);
    return () => {
      window.removeEventListener('beforeunload', cleanup);
      deleteSession(sessionIdRef.current);
    };
  }, []);

  const handleDragOver = (e) => {
    e.preventDefault();
    e.stopPropagation();
  };

  const handleDrop = async (e) => {
    e.preventDefault();
    e.stopPropagation();

    // Use scanner for folders
    if (e.dataTransfer.items && e.dataTransfer.items.length > 0) {
      const scanned = await scanFiles(e.dataTransfer.items);
      handleFilesSelect(scanned);
    } else if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      handleFilesSelect(Array.from(e.dataTransfer.files));
    }
  };

  const handleFilesSelect = async (selectedFiles) => {
    if (!selectedFiles || selectedFiles.length === 0) return;

    const incomingFiles = Array.isArray(selectedFiles) ? selectedFiles : Array.from(selectedFiles);

    // Normalize relativePath for folder input (webkitRelativePath)
    incomingFiles.forEach(file => {
      if (!file.relativePath && file.webkitRelativePath) {
        file.relativePath = file.webkitRelativePath;
      }
    });

    setStatus('uploading');
    setErrorMsg('');
    setResult(null);

    // Optimistically update file list so UI isn't blank during upload
    setFiles(prev => {
      const updated = [...prev];
      incomingFiles.forEach(newFile => {
        const path = newFile.relativePath || newFile.name;
        const idx = updated.findIndex(f => (f.relativePath || f.name) === path);
        if (idx !== -1) {
          updated[idx] = newFile;
        } else {
          updated.push(newFile);
        }
      });
      return updated;
    });

    // Cancel previous upload if any
    if (uploadAbortControllerRef.current) {
      uploadAbortControllerRef.current.abort();
    }
    const controller = new AbortController();
    uploadAbortControllerRef.current = controller;

    try {
      const resp = await uploadFiles(incomingFiles, sessionId, controller.signal);

      let finalPaths = [];
      setBackendFilenames(prev => {
        const updated = [...prev];
        resp.filenames.forEach(newPath => {
          if (!updated.includes(newPath)) {
            updated.push(newPath);
          }
        });
        finalPaths = updated;
        return updated;
      });

      setStatus('ready');

      // Auto-mode logic based on current unique paths
      const isSingleZip = finalPaths.length === 1 && finalPaths[0].toLowerCase().endsWith('.zip');

      if (isSingleZip) {
        setMode('extract');
      } else {
        setMode('compress');
      }

    } catch (err) {
      if (err.name === 'AbortError') {
        console.log('Upload aborted');
        return;
      }
      console.error(err);
      setErrorMsg('Upload failed. Please try again.');
      setStatus('error');
    } finally {
      if (uploadAbortControllerRef.current === controller) {
        uploadAbortControllerRef.current = null;
      }
    }
  };

  const canExtract = () => {
    // Must be single file, ending in zip
    // AND importantly, invalid if we have multiple files
    // Use backendFilenames as truth for processing
    if (backendFilenames.length !== 1) return false;
    return backendFilenames[0].toLowerCase().endsWith('.zip');
  };

  const handleProcess = async () => {
    if (!backendFilenames || backendFilenames.length === 0 || !mode) return;

    setStatus('processing');
    setErrorMsg('');

    try {
      let resp;
      if (mode === 'compress') {
        resp = await compressFiles(backendFilenames, sessionId);
        setResult({ downloadUrl: resp.downloadUrl });
      } else {
        if (!canExtract()) {
          alert("Extraction is only supported for single .zip files.");
          setStatus('ready');
          return;
        }
        resp = await extractFiles(backendFilenames, sessionId, password);
        setResult({ files: resp.files });
        setShowPasswordInput(false); // Success, hide if open
      }
      setStatus('completed');
    } catch (err) {
      console.error(err);

      const resMsg = err.response?.data?.message || err.response?.data || err.message || '';
      const isPasswordError = (err.response && err.response.status === 401) ||
        resMsg.toString().toLowerCase().includes('password');

      // Check for password requirement
      if (isPasswordError) {
        setErrorMsg(typeof resMsg === 'string' ? resMsg : "Password required");
        setShowPasswordInput(true);
        setStatus('ready'); // Go back to ready to allow input
        return;
      }

      setErrorMsg('Processing failed: ' + resMsg);
      setStatus('error');
    }
  };

  const removeIndex = (index) => {
    const updatedFiles = [...files];
    const updatedBackend = [...backendFilenames];

    const fileToRemove = updatedFiles[index];
    const pathToRemove = updatedBackend[index];

    updatedFiles.splice(index, 1);
    updatedBackend.splice(index, 1);

    setFiles(updatedFiles);
    setBackendFilenames(updatedBackend);

    // If it was already uploaded to backend, delete it there too
    if (pathToRemove) {
      deleteFile(sessionId, pathToRemove);
    }

    if (updatedFiles.length === 0) {
      reset();
    } else {
      // Re-evaluate mode based on remaining files
      const isSingleZip = updatedBackend.length === 1 && updatedBackend[0].toLowerCase().endsWith('.zip');
      if (isSingleZip) {
        setMode('extract');
      } else {
        setMode('compress');
      }
    }
  };

  const reset = () => {
    // Cleanup backend session before resetting local state
    if (sessionId) {
      deleteSession(sessionId);
    }

    setFiles([]);
    setBackendFilenames([]);
    setStatus('idle');
    setMode(null);
    setResult(null);
    setErrorMsg('');
    setShowPasswordInput(false);
    setPassword('');
    if (fileInputRef.current) fileInputRef.current.value = '';
    if (folderInputRef.current) folderInputRef.current.value = '';

    // Fresh session for fresh start logic
    const nextId = generateUUID();
    setSessionId(nextId);
    sessionIdRef.current = nextId;

    // Cancel any active upload
    if (uploadAbortControllerRef.current) {
      uploadAbortControllerRef.current.abort();
      uploadAbortControllerRef.current = null;
    }
  };

  const getTotalSize = () => {
    return files.reduce((acc, f) => acc + f.size, 0) / 1024;
  };

  return (
    <div className="app-container">
      <h1>VaultZip</h1>
      <p style={{ color: 'var(--text-muted)', marginBottom: '2rem' }}>
        Securely compress and extract your files.
      </p>

      <div className="glass-card">
        {status === 'idle' && (
          <div
            className="upload-area"
            onDragOver={handleDragOver}
            onDrop={handleDrop}
          >
            <UploadCloud size={64} color="var(--primary)" />
            <p style={{ fontSize: '1.2em', fontWeight: 'bold' }}>Select something to start</p>

            <div style={{ display: 'flex', gap: '1rem', marginTop: '1.5rem', justifyContent: 'center' }}>
              <button onClick={() => fileInputRef.current.click()}>
                <FileText size={18} /> Upload Files
              </button>
              <button className="secondary-btn" onClick={() => folderInputRef.current.click()}>
                <Folder size={18} /> Upload Folder
              </button>
            </div>

            <p style={{ color: 'var(--text-muted)', marginTop: '1rem', fontSize: '0.9em' }}>
              Or drag and drop them here
            </p>
          </div>
        )}

        {/* Input (Note: input type=file doesn't support folders easily without webkitdirectory) */}
        {/* We can have two inputs? Or just standard file input */}
        <input
          type="file"
          multiple
          ref={fileInputRef}
          style={{ display: 'none' }}
          onChange={(e) => handleFilesSelect(e.target.files)}
        />
        <input
          type="file"
          webkitdirectory="true"
          directory="true"
          ref={folderInputRef}
          style={{ display: 'none' }}
          onChange={(e) => handleFilesSelect(e.target.files)}
        />

        {(status === 'uploading' || status === 'ready' || status === 'processing' || status === 'completed' || status === 'error') && (files.length > 0 || status === 'uploading') && (
          <div
            className="active-area"
            onDragOver={handleDragOver}
            onDrop={handleDrop}
          >
            {/* File Info Header */}
            <div className="file-info">
              <div style={{ position: 'relative' }}>
                {/* Icon Logic */}
                {files.some(f => f.relativePath && f.relativePath.includes('/')) ? (
                  <Folder size={24} color="#fbbf24" />
                ) : (
                  <FileArchive size={24} color="#fbbf24" />
                )}

                {files.length > 1 && (
                  <span style={{
                    position: 'absolute', top: -5, right: -5,
                    background: 'var(--primary)', fontSize: '0.6em',
                    borderRadius: '50%', width: '16px', height: '16px',
                    display: 'flex', alignItems: 'center', justifyContent: 'center'
                  }}>
                    {files.length}
                  </span>
                )}
              </div>
              <div style={{ flex: 1, textAlign: 'left' }}>
                <div style={{ fontWeight: 600 }}>
                  {files.length === 1 ? (files[0].relativePath || files[0].name) : `${files.length} items selected`}
                </div>
                <div style={{ fontSize: '0.85em', color: 'var(--text-muted)' }}>
                  Total: {getTotalSize().toFixed(2)} KB
                </div>
              </div>

              {/* Add More Buttons */}
              {(status === 'ready' || status === 'error') && (
                <div style={{ display: 'flex', gap: '0.4rem', marginRight: '0.5rem' }}>
                  <button
                    className="secondary-btn"
                    onClick={() => fileInputRef.current.click()}
                    style={{ padding: '0.4rem' }}
                    title="Add more files"
                  >
                    <Plus size={18} />
                  </button>
                  <button
                    className="secondary-btn"
                    onClick={() => folderInputRef.current.click()}
                    style={{ padding: '0.4rem' }}
                    title="Add more folder"
                  >
                    <Folder size={18} />
                  </button>
                </div>
              )}

              <button
                className="secondary-btn"
                onClick={reset}
                style={{ padding: '0.4rem' }}
                title="Reset"
              >
                <X size={18} />
              </button>
            </div>

            {/* Extended File List (always visible to allow removal) */}
            {status !== 'completed' && (
              <div style={{ textAlign: 'left', fontSize: '0.8em', color: 'var(--text-muted)', marginBottom: '1rem', maxHeight: '150px', overflowY: 'auto', borderTop: '1px solid rgba(255,255,255,0.05)', paddingTop: '1rem' }}>
                {files.map((f, i) => (
                  <div key={i} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', padding: '0.3rem 0' }} className="file-list-item">
                    <FileText size={12} />
                    <span style={{ flex: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                      {f.relativePath || f.name}
                    </span>
                    {(status === 'ready' || status === 'error') && (
                      <button
                        onClick={() => removeIndex(i)}
                        style={{
                          background: 'none', border: 'none', padding: '2px',
                          cursor: 'pointer', color: 'rgba(255,255,255,0.3)',
                          display: 'flex', alignItems: 'center'
                        }}
                        className="remove-btn"
                        title="Remove"
                      >
                        <X size={14} />
                      </button>
                    )}
                  </div>
                ))}
              </div>
            )}

            {status === 'error' && (
              <div style={{ color: '#f87171', background: 'rgba(248,113,113,0.1)', padding: '1rem', borderRadius: '8px', marginBottom: '1rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <AlertCircle size={20} />
                {errorMsg}
              </div>
            )}

            {status === 'uploading' && (
              <div style={{ margin: '1rem 0' }}>
                <p style={{ color: 'var(--primary)', fontWeight: 600, marginBottom: '0.5rem' }}>Uploading files...</p>
                <div className="progress-bar"><div className="progress-fill"></div></div>
              </div>
            )}

            {status === 'ready' && (
              <div>
                <div className="btn-group">
                  <button
                    onClick={() => {
                      // Reset errors when switching modes
                      setErrorMsg('');
                      setShowPasswordInput(false);
                      setPassword('');
                      setMode('compress');
                    }}
                    style={{ opacity: mode === 'compress' ? 1 : 0.5 }}
                  >
                    Compress
                  </button>
                  <button
                    onClick={() => {
                      if (canExtract()) {
                        // Reset errors when switching
                        setErrorMsg('');
                        setShowPasswordInput(false);
                        setPassword('');
                        setMode('extract');
                      }
                    }}
                    style={{
                      opacity: canExtract() ? (mode === 'extract' ? 1 : 0.5) : 0.3,
                      cursor: canExtract() ? 'pointer' : 'not-allowed'
                    }}
                    title={canExtract() ? "Extract archive" : "Extraction only available for single .zip files"}
                  >
                    Extract
                  </button>
                </div>

                {showPasswordInput && (
                  <div style={{ marginTop: '1rem', textAlign: 'left' }}>
                    <label style={{ display: 'block', marginBottom: '0.5rem', fontSize: '0.9em', fontWeight: 600 }}>
                      Archive Password
                    </label>
                    <input
                      type="password"
                      value={password}
                      onChange={(e) => {
                        setPassword(e.target.value);
                        // Clear error when user starts typing
                        if (errorMsg === 'Password required' || errorMsg === 'Invalid password' || errorMsg.includes('password')) {
                          setErrorMsg('');
                        }
                      }}
                      placeholder="Enter password..."
                      style={{
                        width: '100%', padding: '0.6rem',
                        border: '1px solid var(--border)', borderRadius: '6px',
                        background: 'rgba(255,255,255,0.05)', color: 'var(--text)'
                      }}
                    />
                    {errorMsg && (
                      <div style={{ color: '#f87171', fontSize: '0.85em', marginTop: '0.5rem' }}>
                        {errorMsg}
                      </div>
                    )}
                  </div>
                )}

                <div style={{ marginTop: '2rem' }}>
                  <button onClick={handleProcess} style={{ width: '100%' }}>
                    {showPasswordInput ? 'Unlock & Extract' : `Start ${mode === 'compress' ? 'Compression' : 'Extraction'}`}
                  </button>
                </div>
              </div>
            )}

            {status === 'processing' && (
              <div style={{ margin: '2rem 0' }}>
                <p style={{ color: 'var(--primary)' }}>Processing...</p>
                <RefreshCw className="spin" size={32} style={{ animation: 'spin 1s linear infinite' }} />
                <style>{`@keyframes spin { 100% { transform: rotate(360deg); } }`}</style>
              </div>
            )}

            {status === 'completed' && result && (
              <div style={{ marginTop: '1rem' }}>
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '1rem', padding: '1rem', background: 'rgba(50,200,100,0.1)', borderRadius: '8px' }}>
                  <CheckCircle size={48} color="#4ade80" />
                  <h3>Success!</h3>

                  {result.downloadUrl && (
                    <a
                      href={getDownloadUrl(result.downloadUrl)}
                      target="_blank"
                      rel="noopener noreferrer"
                      style={{ textDecoration: 'none' }}
                    >
                      <button style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                        <Download size={20} /> Download ZIP
                      </button>
                    </a>
                  )}

                  {result.files && (
                    <div style={{ width: '100%', textAlign: 'left' }}>
                      <p>Extracted Files:</p>
                      {result.files.length === 0 ? (
                        <p style={{ fontStyle: 'italic', opacity: 0.6 }}>No files found inside archive.</p>
                      ) : (
                        <ul style={{ maxHeight: '150px', overflowY: 'auto', paddingLeft: '1.2rem' }}>
                          {result.files.map((f, i) => (
                            <li key={i}>
                              <a
                                href={getDownloadUrl(f)}
                                className="download-link"
                                target="_blank"
                                rel="noopener noreferrer"
                              >
                                {f.split('/').pop()}
                              </a>
                            </li>
                          ))}
                        </ul>
                      )}
                    </div>
                  )}
                </div>

                <button
                  className="secondary-btn"
                  onClick={reset}
                  style={{ marginTop: '1.5rem', width: '100%' }}
                >
                  Process More Files
                </button>
              </div>
            )}

          </div>
        )}
      </div>
    </div>
  );
}

export default App;
