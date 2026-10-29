import { useEffect, useState, useMemo, useRef } from "react";
import { workbench } from "../wailsjs/go/models";
import { LaunchProjectTerminal } from "../wailsjs/go/main/App";

interface CommandPaletteProps {
  isOpen: boolean;
  onClose: () => void;
  projects: workbench.Project[];
  selectedProject: workbench.Project | null | undefined;
  onSelectProject: (id: string) => void;
  onSelectTab: (tab: any) => void;
  routes: workbench.Route[];
  onTriggerAction?: (actionName: string) => void;
}

interface PaletteItem {
  id: string;
  category: "Navigation" | "Project" | "Terminal" | "Action" | "Route";
  title: string;
  subtitle?: string;
  icon?: string;
  action: () => void;
}

export default function CommandPalette({
  isOpen,
  onClose,
  projects,
  selectedProject,
  onSelectProject,
  onSelectTab,
  routes,
  onTriggerAction,
}: CommandPaletteProps) {
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isOpen) {
      setQuery("");
      setSelectedIndex(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [isOpen]);

  const items = useMemo(() => {
    const list: PaletteItem[] = [];

    // Navigation items
    const tabs: Array<{ id: string; title: string; subtitle: string; icon: string }> = [
      { id: "dashboard", title: "Go to Mission Control Dashboard", subtitle: "Real-time vitals, traffic velocity, query radar & launchpad", icon: "📊" },
      { id: "companion", title: "Go to Laravel Companion", subtitle: "Workspace health, diagnostics, migrations & snapshots", icon: "🚀" },
      { id: "requests", title: "Go to Telemetry & Requests", subtitle: "Clockwork captures, queries, and baselines", icon: "⚡" },
      { id: "tinker", title: "Go to Tinker REPL", subtitle: "Interactive Laravel PHP console", icon: "💬" },
      { id: "database", title: "Go to Database Browser", subtitle: "Eloquent models and typed record editor", icon: "🗄️" },
      { id: "quality", title: "Go to Quality & Audit", subtitle: "Best Practices Doctor & static code reviews", icon: "🩺" },
      { id: "environment", title: "Go to Environment (.env)", subtitle: "Safe .env editing and PHP configuration", icon: "⚙️" },
      { id: "services", title: "Go to Local Services", subtitle: "Mailpit, k6, and Meilisearch profiles", icon: "🛠️" },
      { id: "logs", title: "Go to Laravel Logs", subtitle: "Recent application log streams & stack traces", icon: "📜" },
      { id: "testing", title: "Go to Testing & HTTP", subtitle: "API requests, boundary tests, and Pest generators", icon: "🧪" },
      { id: "settings", title: "Go to Settings", subtitle: "Dependency manager and workspace configuration", icon: "🔧" },
    ];

    tabs.forEach((t) => {
      list.push({
        id: `tab-${t.id}`,
        category: "Navigation",
        title: t.title,
        subtitle: t.subtitle,
        icon: t.icon,
        action: () => {
          onSelectTab(t.id);
          onClose();
        },
      });
    });

    // Terminal launchers (if project selected)
    if (selectedProject) {
      list.push({
        id: "term-powershell",
        category: "Terminal",
        title: `Open PowerShell in ${selectedProject.name}`,
        subtitle: `Injected with PHP ${selectedProject.php} runtime`,
        icon: "💻",
        action: () => {
          LaunchProjectTerminal(selectedProject.id, "powershell");
          onClose();
        },
      });
      list.push({
        id: "term-wt",
        category: "Terminal",
        title: `Open Windows Terminal in ${selectedProject.name}`,
        subtitle: `Injected with PHP ${selectedProject.php} runtime`,
        icon: "🪟",
        action: () => {
          LaunchProjectTerminal(selectedProject.id, "wt");
          onClose();
        },
      });
      list.push({
        id: "term-cmd",
        category: "Terminal",
        title: `Open Command Prompt in ${selectedProject.name}`,
        subtitle: `cmd.exe in ${selectedProject.path}`,
        icon: "⌨️",
        action: () => {
          LaunchProjectTerminal(selectedProject.id, "cmd");
          onClose();
        },
      });
    }

    // Projects list
    projects.forEach((p) => {
      if (p.id !== selectedProject?.id) {
        list.push({
          id: `project-${p.id}`,
          category: "Project",
          title: `Switch to ${p.name}`,
          subtitle: `${p.path} • PHP ${p.php}`,
          icon: "📁",
          action: () => {
            onSelectProject(p.id);
            onClose();
          },
        });
      }
    });

    // Top routes (first 25 if matching)
    if (routes && routes.length > 0) {
      routes.slice(0, 50).forEach((r, idx) => {
        list.push({
          id: `route-${idx}`,
          category: "Route",
          title: `${r.method || "GET"} /${r.uri.replace(/^\//, "")}`,
          subtitle: r.action || r.name || "Closure / Invokable",
          icon: "🔗",
          action: () => {
            onSelectTab("requests");
            onClose();
          },
        });
      });
    }

    // Filter by query
    const q = query.trim().toLowerCase();
    if (!q) return list;
    return list.filter(
      (item) =>
        item.title.toLowerCase().includes(q) ||
        (item.subtitle && item.subtitle.toLowerCase().includes(q)) ||
        item.category.toLowerCase().includes(q)
    );
  }, [query, projects, selectedProject, routes, onSelectProject, onSelectTab, onClose]);

  // Handle keyboard events
  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      } else if (e.key === "ArrowDown") {
        e.preventDefault();
        setSelectedIndex((prev) => (prev + 1 < items.length ? prev + 1 : 0));
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setSelectedIndex((prev) => (prev - 1 >= 0 ? prev - 1 : Math.max(0, items.length - 1)));
      } else if (e.key === "Enter") {
        e.preventDefault();
        if (items[selectedIndex]) {
          items[selectedIndex].action();
        }
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, items, selectedIndex, onClose]);

  if (!isOpen) return null;

  return (
    <div className="palette-backdrop" onClick={onClose}>
      <div className="palette-modal" onClick={(e) => e.stopPropagation()}>
        <div className="palette-search-bar">
          <span className="palette-search-icon">🔍</span>
          <input
            ref={inputRef}
            type="text"
            className="palette-input"
            placeholder="Type a command, route, tab, or project name... (↑↓ to navigate, Enter to run, Esc to close)"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setSelectedIndex(0);
            }}
          />
          <kbd className="palette-kbd">Esc</kbd>
        </div>

        <div className="palette-results">
          {items.length === 0 ? (
            <div className="palette-empty">No matching commands or routes found.</div>
          ) : (
            items.map((item, index) => {
              const isSelected = index === selectedIndex;
              return (
                <div
                  key={item.id}
                  className={`palette-item ${isSelected ? "selected" : ""}`}
                  onClick={item.action}
                  onMouseEnter={() => setSelectedIndex(index)}
                >
                  <span className="palette-item-icon">{item.icon || "•"}</span>
                  <div className="palette-item-content">
                    <div className="palette-item-title">
                      {item.title}
                      <span className="palette-item-tag">{item.category}</span>
                    </div>
                    {item.subtitle && <div className="palette-item-subtitle">{item.subtitle}</div>}
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}
