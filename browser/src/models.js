const DEFAULTS={
  decision:{name:"GLiNER 2.5 (optional)",url:"https://huggingface.co/onnx-community/gliner_small-v2.1/resolve/main/onnx/model.onnx"},
  language:{name:"Qwen3-0.6B (optional)",url:"https://huggingface.co/onnx-community/Qwen3-0.6B-ONNX/resolve/main/onnx/model_q4.onnx"}
};
export class ModelManager{
  constructor(onProgress=()=>{}){this.onProgress=onProgress}
  async cached(kind){const spec=DEFAULTS[kind];if(!spec)return false;const c=await caches.open("agame-models-v1");return !!(await c.match(spec.url))}
  async load(kind){const spec=DEFAULTS[kind];if(!spec)throw new Error("Unknown model "+kind);const cache=await caches.open("agame-models-v1");const hit=await cache.match(spec.url);if(hit){this.onProgress({kind,name:spec.name,state:"ready",cached:true});return true}this.onProgress({kind,name:spec.name,state:"connecting",loaded:0,total:0});const res=await fetch(spec.url,{mode:"cors"});if(!res.ok)throw new Error(spec.name+" download failed: HTTP "+res.status);const total=Number(res.headers.get("content-length"))||0;if(!res.body){await cache.put(spec.url,res.clone());this.onProgress({kind,name:spec.name,state:"ready",loaded:total,total});return true}const reader=res.body.getReader(),chunks=[];let loaded=0;for(;;){const {done,value}=await reader.read();if(done)break;chunks.push(value);loaded+=value.byteLength;this.onProgress({kind,name:spec.name,state:"downloading",loaded,total})}const blob=new Blob(chunks,{type:res.headers.get("content-type")||"application/octet-stream"});await cache.put(spec.url,new Response(blob,{headers:{"content-type":blob.type,"content-length":String(blob.size)}}));this.onProgress({kind,name:spec.name,state:"ready",loaded:blob.size,total:blob.size});return true}
  async remove(kind){const spec=DEFAULTS[kind];const c=await caches.open("agame-models-v1");await c.delete(spec.url);this.onProgress({kind,name:spec.name,state:"not-loaded"})}
}
export {DEFAULTS};