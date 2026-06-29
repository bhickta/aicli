import { createRouter, createWebHashHistory, type RouteRecordRaw } from "vue-router";
import ChatView from "../views/ChatView.vue";
import JobsView from "../views/JobsView.vue";
import ProvidersView from "../views/ProvidersView.vue";
import SettingsView from "../views/SettingsView.vue";
import StudyView from "../views/StudyView.vue";
import ToolsView from "../views/ToolsView.vue";
import WorkflowsView from "../views/WorkflowsView.vue";
import ZettelView from "../views/ZettelView.vue";

export type AppRouteGroup = "Work" | "System";

export interface AppRouteMeta {
  label: string;
  description: string;
  group: AppRouteGroup;
  nav?: boolean;
}

export const routeDefinitions = [
  {
    path: "/chat",
    name: "chat",
    component: ChatView,
    meta: { label: "Chat", description: "Ask a configured model", group: "Work", nav: true },
  },
  {
    path: "/study/:section?",
    name: "study",
    component: StudyView,
    meta: { label: "Study", description: "UPSC copies, recall, lectures", group: "Work", nav: true },
  },
  {
    path: "/workflows/:category?",
    name: "workflows",
    component: WorkflowsView,
    meta: { label: "Workflows", description: "Documents, media, automation", group: "Work", nav: true },
  },
  {
    path: "/zettel/:mode?",
    name: "zettel",
    component: ZettelView,
    meta: { label: "Zettel", description: "Vault merge and review", group: "Work", nav: true },
  },
  {
    path: "/jobs/:filter?",
    name: "jobs",
    component: JobsView,
    meta: { label: "Jobs", description: "Queue, progress, history", group: "System", nav: true },
  },
  {
    path: "/providers",
    name: "providers",
    component: ProvidersView,
    meta: { label: "Providers", description: "Models and health", group: "System", nav: true },
  },
  {
    path: "/tools",
    name: "tools",
    component: ToolsView,
    meta: { label: "Tools", description: "Local binaries", group: "System", nav: true },
  },
  {
    path: "/settings",
    name: "settings",
    component: SettingsView,
    meta: { label: "Settings", description: "Defaults and paths", group: "System", nav: true },
  },
  { path: "/", redirect: "/chat" },
  { path: "/:pathMatch(.*)*", redirect: "/chat" },
] satisfies RouteRecordRaw[];

export const primaryRoutes = routeDefinitions.filter((route) => route.meta?.nav);

export const router = createRouter({
  history: createWebHashHistory(),
  routes: routeDefinitions,
});
