import argparse
import base64
import inspect
import io
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import threading
import time
from typing import Any, Dict, List, Optional

import torch
from diffusers import DiffusionPipeline


def parse_size(value: Optional[str]) -> tuple[int, int]:
    if not value:
        return 1024, 1024
    parts = value.lower().split("x", 1)
    if len(parts) != 2:
        raise ValueError("size must be WIDTHxHEIGHT")
    try:
        width = int(parts[0])
        height = int(parts[1])
    except ValueError as exc:
        raise ValueError("size must be WIDTHxHEIGHT") from exc
    if width <= 0 or height <= 0:
        raise ValueError("size must be positive")
    return width, height


def detect_device() -> tuple[str, torch.dtype]:
    if torch.cuda.is_available():
        return "cuda", torch.float16
    if getattr(torch.backends, "mps", None) is not None and torch.backends.mps.is_available():
        return "mps", torch.float16
    return "cpu", torch.float32


class Worker:
    def __init__(self, model_dir: str, model_name: str) -> None:
        self.model_dir = model_dir
        self.model_name = model_name
        self.device, self.dtype = detect_device()
        self.pipeline = None
        self.lock = threading.Lock()

    def load(self) -> None:
        kwargs: Dict[str, Any] = {"torch_dtype": self.dtype, "local_files_only": True}
        self.pipeline = DiffusionPipeline.from_pretrained(self.model_dir, **kwargs)
        if self.device == "cuda":
            self.pipeline = self.pipeline.to("cuda")
            if hasattr(self.pipeline, "enable_model_cpu_offload"):
                self.pipeline.enable_model_cpu_offload()
        elif self.device == "mps":
            self.pipeline = self.pipeline.to("mps")
        else:
            self.pipeline = self.pipeline.to("cpu")
        if hasattr(self.pipeline, "enable_attention_slicing"):
            self.pipeline.enable_attention_slicing()
        if hasattr(self.pipeline, "enable_vae_tiling"):
            self.pipeline.enable_vae_tiling()

    def generate(self, req: Dict[str, Any]) -> Dict[str, Any]:
        if self.pipeline is None:
            raise RuntimeError("pipeline is not loaded")
        width, height = parse_size(req.get("size"))
        count = max(1, min(int(req.get("n") or 1), 4))
        generator = None
        if req.get("seed") is not None:
            generator = torch.Generator(device=self.device if self.device != "mps" else "cpu").manual_seed(int(req["seed"]))
        kwargs: Dict[str, Any] = {
            "prompt": req.get("prompt", ""),
            "width": width,
            "height": height,
            "num_images_per_prompt": count,
        }
        if req.get("negative_prompt"):
            kwargs["negative_prompt"] = req["negative_prompt"]
        if req.get("steps"):
            kwargs["num_inference_steps"] = int(req["steps"])
        if req.get("cfg_scale") is not None:
            signature = inspect.signature(self.pipeline.__call__)
            if "true_cfg_scale" in signature.parameters:
                kwargs["true_cfg_scale"] = float(req["cfg_scale"])
            elif "guidance_scale" in signature.parameters:
                kwargs["guidance_scale"] = float(req["cfg_scale"])
        if generator is not None:
            kwargs["generator"] = generator
        with self.lock:
            result = self.pipeline(**kwargs)
        data: List[Dict[str, str]] = []
        for image in result.images:
            buf = io.BytesIO()
            image.save(buf, format="PNG")
            encoded = base64.b64encode(buf.getvalue()).decode("ascii")
            data.append({"b64_json": encoded})
        return {"created": int(time.time()), "data": data}


class Handler(BaseHTTPRequestHandler):
    worker: Worker

    def log_message(self, fmt: str, *args: Any) -> None:
        return

    def write_json(self, status: int, payload: Dict[str, Any]) -> None:
        data = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self) -> None:
        if self.path != "/health":
            self.write_json(404, {"error": "not found"})
            return
        self.write_json(200, {"status": "ok", "device": self.worker.device, "model": self.worker.model_name})

    def do_POST(self) -> None:
        if self.path != "/generate":
            self.write_json(404, {"error": "not found"})
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            req = json.loads(self.rfile.read(length).decode("utf-8"))
            if not req.get("prompt"):
                self.write_json(400, {"error": "prompt is required"})
                return
            self.write_json(200, self.worker.generate(req))
        except ValueError as exc:
            self.write_json(400, {"error": str(exc)})
        except Exception as exc:
            self.write_json(500, {"error": str(exc)})


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--model-dir", required=True)
    parser.add_argument("--model-name", required=True)
    parser.add_argument("--port", type=int, required=True)
    args = parser.parse_args()

    worker = Worker(args.model_dir, args.model_name)
    worker.load()
    Handler.worker = worker
    server = ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
    server.serve_forever()


if __name__ == "__main__":
    main()

