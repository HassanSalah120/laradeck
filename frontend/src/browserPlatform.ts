// Preserve generated Wails bindings: browser delivery supplies the same calling
// convention, while desktop delivery continues to use Wails' native bridge.
export async function initializePlatform() {
  const platform = window as unknown as {go?: {main: {App: unknown}}; runtime?: unknown};
  if (platform.go?.main?.App) return;
  const session = new URLSearchParams(location.hash.slice(1)).get('session');
  if (session) {
    history.replaceState(null, '', location.pathname);
    const response = await fetch('/api/session', {method: 'POST', headers: {Authorization: `Bearer ${session}`}});
    if (!response.ok) throw new Error('Dashboard session could not be established. Open it from LaraDeck.');
  }
  const call = async (method: string, args: unknown[]) => {
    const response = await fetch('/api/call', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({method,args})});
    if (!response.ok) throw new Error(await response.text());
    const result = await response.json();
    if (result.error) throw new Error(result.error);
    return result.result;
  };
  platform.go = {main:{App:new Proxy({}, {get:(_target,method:string) => (...args:unknown[]) => {
    if (method === 'OpenProject' || method === 'OpenCompanionDashboard' || method === 'OpenDependencyWebsite') {
      const page = window.open('about:blank', '_blank');
      return call('BrowserOpenTarget',[method,args]).then((url:string) => { if(page) {page.opener=null; page.location.href=url;} }).catch(error => {page?.close(); throw error;});
    }
    return call(method,args);
  }})}};
  type Listener = {callback: (...args: unknown[]) => void; remaining: number};
  const listeners = new Map<string, Set<Listener>>();
  const events = new EventSource('/api/events');
  events.onmessage = event => {
    const name = JSON.parse(event.data);
    for (const listener of listeners.get(name) || []) {
      listener.callback();
      if (listener.remaining > 0 && --listener.remaining === 0) listeners.get(name)?.delete(listener);
    }
  };
  platform.runtime = {
    EventsOnMultiple: (name:string, callback:Listener['callback'], count:number) => {
      const listener = {callback,remaining:count};
      if (!listeners.has(name)) listeners.set(name,new Set());
      listeners.get(name)!.add(listener);
      return () => listeners.get(name)?.delete(listener);
    },
    EventsOff: (...names:string[]) => names.forEach(name => listeners.delete(name)),
    EventsOffAll: () => listeners.clear(),
    ClipboardSetText: async (text:string) => {await navigator.clipboard.writeText(text);return true;},
    BrowserOpenURL: (url:string) => window.open(url,'_blank','noopener'),
  };
  await call('GetState',[]);
}
