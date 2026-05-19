import { AUDIO_WORKFLOWS } from "./workflows/audio.js";
import { DOCUMENT_WORKFLOWS } from "./workflows/documents.js";
import { IMAGE_WORKFLOWS } from "./workflows/images.js";
import { NEWS_WORKFLOWS } from "./workflows/news.js";
import { STUDY_WORKFLOWS } from "./workflows/study.js";
import { VIDEO_WORKFLOWS } from "./workflows/video.js";

export const WORKFLOW_CATEGORIES = ["Study", "Documents", "Images", "Audio", "Video", "News"];

export const WORKFLOW_DEFINITIONS = [
  ...STUDY_WORKFLOWS,
  ...DOCUMENT_WORKFLOWS,
  ...IMAGE_WORKFLOWS,
  ...AUDIO_WORKFLOWS,
  ...VIDEO_WORKFLOWS,
  ...NEWS_WORKFLOWS,
];
