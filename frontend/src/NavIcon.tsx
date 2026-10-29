const paths = {
  activity: "M3 12h4l3-8 4 16 3-8h4",
  logs: "M7 3h10l3 3v15H4V3h3m1 5h8m-8 4h8m-8 4h5",
  terminal: "m5 6 6 6-6 6m8 0h6",
  database: "M20 6c0 2-4 3-8 3S4 8 4 6s4-3 8-3 8 1 8 3ZM4 6v12c0 2 4 3 8 3s8-1 8-3V6M4 12c0 2 4 3 8 3s8-1 8-3",
  play: "m8 4 12 8-12 8V4Z",
  shield: "m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6l8-3Zm-4 9 3 3 5-6",
  test: "M9 3h6m-5 0v6l-6 10c-1 1 0 2 1 2h14c1 0 2-1 1-2L14 9V3M7 15h10",
  sliders: "M4 7h9m4 0h3M4 17h3m4 0h9M13 4v6M7 14v6",
  mail: "M3 5h18v14H3V5Zm0 1 9 7 9-7",
  box: "m12 3 9 5v9l-9 5-9-5V8l9-5Zm-9 5 9 5 9-5m-9 5v9M8 5l9 5",
  graph: "M12 4v7M5 16v-5h14v5M9 2h6v4H9V2ZM2 17h6v5H2v-5Zm14 0h6v5h-6v-5Z",
  dashboard: "M3 3h8v8H3V3Zm10 0h8v5h-8V3Zm0 9h8v9h-8v-9ZM3 15h8v6H3v-6Z",
  settings: "M4 5h16v14H4V5Zm0 5h16m-9 0v9M7 7.5h.01m3 0h.01M14 14h3m-3 3h3",
};

export function NavIcon({ name }: { name: keyof typeof paths }) {
  return <svg className="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={paths[name]} /></svg>;
}
