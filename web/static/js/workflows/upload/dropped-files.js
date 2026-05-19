export async function collectDroppedFiles(dataTransfer) {
  const items = Array.from(dataTransfer?.items || []);
  const entries = [];
  const readers = items
    .map((item) => (typeof item.webkitGetAsEntry === "function" ? item.webkitGetAsEntry() : null))
    .filter(Boolean);
  if (readers.length) {
    for (const entry of readers) {
      await collectEntryFiles(entry, "", entries);
    }
  }
  if (!entries.length) {
    Array.from(dataTransfer?.files || []).forEach((file) => {
      entries.push({ file, relativePath: file.webkitRelativePath || file.name });
    });
  }
  return entries;
}

function collectEntryFiles(entry, prefix, out) {
  return new Promise((resolve, reject) => {
    if (entry.isFile) {
      entry.file(
        (file) => {
          out.push({ file, relativePath: `${prefix}${file.name}` });
          resolve();
        },
        reject,
      );
      return;
    }
    if (!entry.isDirectory) {
      resolve();
      return;
    }
    const reader = entry.createReader();
    const directoryPrefix = `${prefix}${entry.name}/`;
    const readBatch = () => {
      reader.readEntries(
        async (children) => {
          if (!children.length) {
            resolve();
            return;
          }
          try {
            for (const child of children) {
              await collectEntryFiles(child, directoryPrefix, out);
            }
            readBatch();
          } catch (error) {
            reject(error);
          }
        },
        reject,
      );
    };
    readBatch();
  });
}
