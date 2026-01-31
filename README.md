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
- **Modern UI**: Polished, dark-mode interface with glassmorphism effects and responsive animations.

## Project Structure

- `backend/`: Go source code
  - `main.go`: Entry point & Router
  - `handlers/`: API Route handlers (Upload, Compress, Extract, Download)
  - `services/`: Core logic (ZIP processing)
- `frontend/`: React Vite application
  - `src/App.jsx`: Main UI Logic & State
  - `src/services/api.js`: Backend communication Layer

## Prerequisites

- [Go](https://go.dev/) (1.21+)
- [Node.js](https://nodejs.org/) (20+)

## Getting Started

### 1. Start the Backend

```bash
cd backend
go run main.go
```
The server will start at `http://localhost:8080`.

### 2. Start the Frontend

Open a new terminal:

```bash
cd frontend
npm install # Required the first time or after cloning
npm run dev
```
Open `http://localhost:5173` in your browser.

## API Endpoints

- `POST /upload`: Stream files/folders to the session.
- `POST /compress`: Bundle selected items into a ZIP.
- `POST /extract`: Unpack a ZIP (supports password parameter).
- `GET /download/*`: Securely retrieve processed files.
