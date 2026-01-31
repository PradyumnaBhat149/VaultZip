// Utility to scan files and directories from DataTransferItem

export async function scanFiles(items) {
    const files = [];

    // Helper to read entries from a DirectoryReader
    const readEntries = (reader) => {
        return new Promise((resolve, reject) => {
            const allEntries = [];
            const read = () => {
                reader.readEntries((entries) => {
                    if (entries.length > 0) {
                        allEntries.push(...entries);
                        read(); // Continue reading
                    } else {
                        resolve(allEntries);
                    }
                }, reject);
            };
            read();
        });
    };

    // Helper to scan a single entry recursively
    const scanEntry = async (entry, path = '') => {
        if (entry.isFile) {
            return new Promise((resolve, reject) => {
                entry.file((file) => {
                    // We attach the full relative path to the file object
                    // This is key for the backend to reconstruct structure
                    // entry.fullPath usually has leading slash, e.g. /folder/file.txt
                    const fullPath = path + entry.name; // or entry.fullPath
                    // Actually webkit entry.fullPath is reliable
                    // We want relative path without leading slash
                    let relPath = entry.fullPath;
                    if (relPath.startsWith('/')) relPath = relPath.substring(1);

                    // Override/Attach property
                    // We can't easily override file.name, but we can attach a property
                    Object.defineProperty(file, 'relativePath', {
                        value: relPath,
                        writable: true
                    });

                    files.push(file);
                    resolve();
                }, reject);
            });
        } else if (entry.isDirectory) {
            const reader = entry.createReader();
            const entries = await readEntries(reader);
            for (const childEntry of entries) {
                await scanEntry(childEntry, path + entry.name + '/');
            }
        }
    };

    const entryPromises = [];
    for (let i = 0; i < items.length; i++) {
        const item = items[i];
        if (item.webkitGetAsEntry) {
            const entry = item.webkitGetAsEntry();
            if (entry) {
                entryPromises.push(scanEntry(entry));
            }
        } else if (item.kind === 'file') {
            const file = item.getAsFile();
            if (file) files.push(file);
        }
    }

    await Promise.all(entryPromises);
    return files;
}
