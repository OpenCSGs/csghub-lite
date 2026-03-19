import { ComponentChildren } from "preact";
import { useLocation } from "preact-iso";
import { t, locale } from "../i18n";

const navKeys = [
  { path: "/", key: "nav.dashboard", icon: DashboardIcon },
  { path: "/marketplace", key: "nav.marketplace", icon: MarketplaceIcon },
  { path: "/library", key: "nav.library", icon: LibraryIcon },
  { path: "/chat", key: "nav.chat", icon: ChatIcon },
];

function SettingsIcon({ active }: { active: boolean }) {
  return (
    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke={active ? "currentColor" : "#9CA3AF"} stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  );
}

export function Layout({ children }: { children: ComponentChildren }) {
  const { path } = useLocation();
  void locale.value;

  return (
    <div class="flex h-screen overflow-hidden">
      <aside class="w-52 flex-shrink-0 border-r border-gray-200 bg-white flex flex-col">
        <div class="flex items-center gap-2 px-5 py-5">
          <img src="/favicon.svg" alt="CSGHub Lite" class="w-8 h-8" />
          <span class="font-semibold text-base text-gray-900">CSGHub Lite</span>
        </div>
        <nav class="flex-1 px-3 space-y-1 mt-2">
          {navKeys.map((item) => {
            const active = path === item.path || (item.path !== "/" && path.startsWith(item.path));
            return (
              <a
                key={item.path}
                href={item.path}
                class={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                  active
                    ? "bg-indigo-50 text-indigo-700"
                    : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
                }`}
              >
                <item.icon active={active} />
                {t(item.key)}
              </a>
            );
          })}
        </nav>
        {(() => {
          const active = path === "/settings";
          return (
            <a
              href="/settings"
              class={`flex items-center gap-3 mx-3 mb-4 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                active
                  ? "bg-indigo-50 text-indigo-700"
                  : "text-gray-600 hover:bg-gray-50 hover:text-gray-900"
              }`}
            >
              <SettingsIcon active={active} />
              {t("nav.settings")}
            </a>
          );
        })()}
      </aside>
      <main class="flex-1 overflow-auto bg-gray-50 flex flex-col">
        <div class="flex-1">{children}</div>
        <div class="py-3 text-xs text-gray-400 text-center">
          &copy; OpenCSG &middot; Powered By OpenCSG
        </div>
      </main>
    </div>
  );
}

function DashboardIcon({ active }: { active: boolean }) {
  return (
    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke={active ? "currentColor" : "#9CA3AF"} stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zm10 0a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zm10 0a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
    </svg>
  );
}

function MarketplaceIcon({ active }: { active: boolean }) {
  return (
    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke={active ? "currentColor" : "#9CA3AF"} stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M3 3h2l.4 2M7 13h10l4-8H5.4M7 13L5.4 5M7 13l-2.293 2.293c-.63.63-.184 1.707.707 1.707H17m0 0a2 2 0 100 4 2 2 0 000-4zm-8 2a2 2 0 100 4 2 2 0 000-4z" />
    </svg>
  );
}

function LibraryIcon({ active }: { active: boolean }) {
  return (
    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke={active ? "currentColor" : "#9CA3AF"} stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
    </svg>
  );
}

function ChatIcon({ active }: { active: boolean }) {
  return (
    <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke={active ? "currentColor" : "#9CA3AF"} stroke-width="2">
      <path stroke-linecap="round" stroke-linejoin="round" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
    </svg>
  );
}
