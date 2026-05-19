import { AUDIO_WORKFLOWS } from "./audio.js";
import { DOCUMENT_WORKFLOWS } from "./documents.js";
import { IMAGE_WORKFLOWS } from "./images.js";
import { NEWS_WORKFLOWS } from "./news.js";
import { STUDY_WORKFLOWS } from "./study.js";
import { VIDEO_WORKFLOWS } from "./video.js";

export const WORKFLOW_CATEGORIES = ["Study", "Documents", "Images", "Audio", "Video", "News"];

export const WORKFLOW_DEFINITIONS = [
  ...STUDY_WORKFLOWS,
  ...DOCUMENT_WORKFLOWS,
  ...IMAGE_WORKFLOWS,
  ...AUDIO_WORKFLOWS,
  ...VIDEO_WORKFLOWS,
  ...NEWS_WORKFLOWS,
];
