export interface DropEntry {
  file: File;
  relativePath: string;
}

export async function collectDroppedFiles(dataTransfer: DataTransfer | null): Promise<DropEntry[]> {
  const items = Array.from(dataTransfer?.items || []);
  const entries: DropEntry[] = [];
  const readers = items
    .map((item) => {
      const legacyItem = item as DataTransferItem & { webkitGetAsEntry?: () => unknown };
      return typeof legacyItem.webkitGetAsEntry === "function" ? legacyItem.webkitGetAsEntry() : null;
    })
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

function collectEntryFiles(entry: unknown, prefix: string, out: DropEntry[]): Promise<void> {
  const item = entry as {
    isFile?: boolean;
    isDirectory?: boolean;
    name: string;
    file?: (success: (file: File) => void, failure: (error: Error) => void) => void;
    createReader?: () => { readEntries: (success: (children: unknown[]) => void, failure: (error: Error) => void) => void };
  };
  return new Promise((resolve, reject) => {
    if (item.isFile && item.file) {
      item.file((file) => {
        out.push({ file, relativePath: `${prefix}${file.name}` });
        resolve();
      }, reject);
      return;
    }
    if (!item.isDirectory || !item.createReader) {
      resolve();
      return;
    }
    const reader = item.createReader();
    const directoryPrefix = `${prefix}${item.name}/`;
    const readBatch = () => {
      reader.readEntries(async (children) => {
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
      }, reject);
    };
    readBatch();
  });
}
