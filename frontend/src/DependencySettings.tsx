import {useEffect,useState} from "react";
import {CancelDependencyInstall,InstallDependency,ChooseDependencyBinary,OpenDependencyWebsite} from "../wailsjs/go/main/App";
import {dependencies,workbench} from "../wailsjs/go/models";

type Props={items:dependencies.Status[];installation?:workbench.DependencyInstallation;project?:workbench.Project;trusted:boolean;busy:string;act:(label:string,action:()=>Promise<unknown>)=>Promise<void>};
export default function DependencySettings({items,installation,project,trusted,busy,act}:Props){
 const [now,setNow]=useState(Date.now());
 const [cancelError,setCancelError]=useState("");
 useEffect(()=>{if(!installation?.running)return;const timer=setInterval(()=>setNow(Date.now()),1000);return()=>clearInterval(timer)},[installation?.running]);
 const elapsed=installation?.startedAt?Math.max(0,Math.floor((now-Date.parse(installation.startedAt))/1000)):0;
 const [installing,setInstalling]=useState("");
 const [result,setResult]=useState("");
 useEffect(()=>setResult(""),[project?.id]);
 const groups=[{scope:"app",title:"Workbench tools",description:"Verified downloads into Workbench's own tools folder. No global PATH changes or administrator access."},{scope:"project",title:"Selected project packages",description:project?`${project.name} · ${project.path}. Composer changes this project's development dependencies and may run its Composer scripts. Project trust is required.`:"Select a Laravel project to install development packages."},{scope:"system",title:"System installers",description:"Opens a visible WinGet/vendor installer. Review licenses and administrator prompts there. Database installers may create Windows services, ask for credentials, and occupy default ports. Workbench will not stop or adopt those services. Finish the installer, then recheck; restart Workbench if PATH changed."},{scope:"global",title:"User-wide Composer tools",description:"Installs into Composer's global directory in your Windows user profile. Installing a tunnel CLI does not authenticate it or open a public tunnel."}];
 return <section className="dependency-settings" aria-label="Dependency setup">
  <div className="workbench-card"><h3>Dependencies &amp; tool installation</h3><p>Detect, install, or select existing tools. Installing a service binary does not start it.</p>
  {result&&<p role="status">{result}</p>}</div>
  {installation?.id&&<div className="workbench-card" aria-label="Installation progress">
   <h3>{installation.name}: {installation.running?`running · ${elapsed}s`:installation.state}</h3>
   <p role="status">{installation.message}</p>
   {installation.running&&<>{["project","global"].includes(items.find(item=>item.id===installation.id)?.scope||"")&&<p>Composer may still be resolving packages, generating autoload files, running project scripts or checking advisories after the package appears installed.</p>}
   <button className="btn-toolbar" onClick={async()=>{setCancelError("");try{await CancelDependencyInstall()}catch(error){setCancelError(String(error))}}}>Cancel installation</button></>}
   {cancelError&&<p role="alert">{cancelError}</p>}
   {!!installation.output?.length&&<pre className="installation-output" aria-label="Recent Composer output">{installation.output.join("\n")}</pre>}
  </div>}
  {groups.map(group=><div className="workbench-card" key={group.scope}>
   <h3>{group.title}</h3><p className="empty-text">{group.description}</p>
   {group.scope==="project"&&project&&!trusted&&<p>Trust the selected project before installing its packages.</p>}
   {items.filter(item=>item.scope===group.scope).map(item=>{
    const working=installing===item.id||item.state==="installing"||(installation?.running&&installation.id===item.id&&(item.scope!=="project"||installation.projectId===project?.id));
    const installed=item.state==="installed";
    return <div className="dependency-row" key={item.id}>
     <div><strong>{item.name}</strong> <span className={`workbench-badge ${installed?"badge-good":item.state==="failed"?"badge-error":"badge-info"}`}>{working?"Installing":item.state.replaceAll("_"," ")}</span>
      {item.version&&<small> {item.version}</small>}
      <p>{item.note}</p>{item.path&&<code className="dependency-path">{item.path}</code>}
      {item.message&&<p role="status">{item.message}</p>}
      {working&&<progress aria-label={`Installing ${item.name}`} max={1} value={item.progress>0?item.progress:undefined}/>}
     </div>
     <div className="dependency-actions">
      <button className="btn-tiny primary" disabled={!!busy||installed||!item.canInstall||working} onClick={()=>act(`Install ${item.name}`,async()=>{
       setInstalling(item.id);setResult("");
       try{const status=await InstallDependency(item.id,project?.id||"");setResult(status.message);}finally{setInstalling("");}
      })}>{item.id==="clockwork"&&item.state==="incomplete"?"Repair Clockwork setup":group.scope==="system"?"Open system installer":group.scope==="project"?"Install into project":group.scope==="global"?"Install for my user":"Install tool"}</button>
      {group.scope!=="project"&&<button className="btn-tiny" disabled={!!busy} onClick={()=>act(`Choose ${item.name}`,()=>ChooseDependencyBinary(item.id))}>Choose existing binary</button>}
      <button className="btn-tiny" disabled={!!busy} onClick={()=>act(`Open ${item.name} website`,()=>OpenDependencyWebsite(item.id))}>Official website</button>
     </div>
    </div>;
   })}
  </div>)}
 </section>;
}
