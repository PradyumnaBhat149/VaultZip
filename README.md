# VaultZip

A modern, high-performance file compression and extraction utility. Built with **Go** and **React**.

## Features

- **Rebranded Identity**: Fast and secure processing under the **VaultZip** brand.
- **Smart Uploads**:
  - Drag & Drop support for files and folders.
  - Dedicated **Upload Folder** picker.
  - Automatic duplicate detection and updating.
- **Archive Management**:
  - **Compress**: Bundle multiple files and nested folder structures into `.zip` archives.
  - **Extract**: Unpack `.zip` archives (including password-protected AES-256 archives).
  - **Terminology**: User-friendly "Extract" nomenclature throughout the app.
- **Pre-Processing Control**:
  - Remove individual files or folders from your selection before starting.
  - Real-time size calculation and item counting.
- **Privacy & Storage Efficiency**:
  - **Instant Cleanup**: Session data and processed files are deleted immediately on reset or page refresh.
  - **Upload Cancellation**: Abort active uploads instantly to save bandwidth.
  - **Auto-Maintenance**: Background worker clears stale files older than 30 minutes every 10 minutes.
- **Modern & Responsive**: 
  - Glassmorphic dark-mode UI.
  - Fully responsive layout optimized for mobile and desktop.

## Project Structure

- `backend/`: Go source code
  - `main.go`: Router & Background cleanup worker
  - `handlers/`: API handlers for lifecycle management
  - `services/`: Core logic for AES-256 ZIP processing
- `frontend/`: React Vite application
  - `src/App.jsx`: Main UI with lifecycle cleanup hooks
  - `src/services/api.js`: Communication with AbortController support

## Prerequisites

- [Go](https://go.dev/) (1.21+)
- [Node.js](https://nodejs.org/) (20+)

## Deployment

VaultZip is designed for modern cloud platforms:
- **Backend**: Recommended for [Render.com](https://render.com/) (Go Runtime).
- **Frontend**: Recommended for [Vercel](https://vercel.com/) (Vite Build).

### Environment Variables
- **Backend**: `PORT` (Dynamic binding for Render).
- **Frontend**: `VITE_API_BASE_URL` (Points to your deployed backend URL).

## Getting Started

### 1. Start the Backend
```bash
cd backend
go run main.go
```
Defaults to `http://localhost:8080`.

### 2. Start the Frontend
```bash
cd frontend
npm install
npm run dev
```

## API Endpoints

- `POST /upload`: Stream files/folders to the server.
- `POST /compress`: Bundle items into a ZIP.
- `POST /extract`: Unpack a ZIP (supports password).
- `GET /download/*`: Securely retrieve results.
- `DELETE /session`: Instant wipe of all session-related files.
- `DELETE /file`: Remove a specific file from a session.
