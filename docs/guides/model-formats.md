# 模型格式

csghub-lite 支持下载和管理多种格式的模型，但仅 GGUF 格式可直接用于本地推理。

## 格式对照

| 格式 | 下载 | 推理 | 说明 |
|------|------|------|------|
| GGUF | 支持 | 支持 | llama.cpp 原生格式，量化后体积小、推理快 |
| SafeTensors | 支持 | 不支持（需转换） | PyTorch 生态常用格式 |

## GGUF 格式

GGUF（General GGML Universal Format）是 llama.cpp 项目的模型格式。特点：

- **单文件**: 模型权重、词表、配置合并在一个 `.gguf` 文件中
- **量化**: 支持多种量化精度（Q4_0、Q4_K_M、Q5_K_M、Q8_0、F16 等）
- **即用**: csghub-lite 可直接加载推理

### 量化级别参考

| 量化 | 说明 | 体积 | 质量 |
|------|------|------|------|
| Q4_0 | 4-bit 基础量化 | 最小 | 一般 |
| Q4_K_M | 4-bit K-quant medium | 小 | 较好 |
| Q5_K_M | 5-bit K-quant medium | 中等 | 好 |
| Q8_0 | 8-bit 量化 | 较大 | 很好 |
| F16 | 16-bit 半精度 | 大 | 原始精度 |

选择建议：

- 内存有限（8GB）: Q4_K_M
- 内存充裕（16GB+）: Q8_0
- 追求精度: F16

## SafeTensors 格式

SafeTensors 是 Hugging Face 推出的模型存储格式。csghub-lite 支持下载但不支持直接推理。

### 转换为 GGUF

使用 llama.cpp 提供的转换工具：

```bash
# 克隆 llama.cpp（如未安装）
git clone https://github.com/ggml-org/llama.cpp
cd llama.cpp

# 安装 Python 依赖
pip install -r requirements.txt

# 转换（默认 f16 精度）
python convert_hf_to_gguf.py /path/to/safetensors/model

# 进一步量化（可选）
./llama-quantize /path/to/model-f16.gguf /path/to/model-q4_k_m.gguf q4_k_m
```

转换完成后，将 `.gguf` 文件放入模型目录即可：

```bash
cp model-q4_k_m.gguf ~/.csghub-lite/models/namespace/name/
```

## 如何选择模型

在 CSGHub 上搜索模型时：

```bash
# 搜索可直接推理的 GGUF 模型
csghub-lite search "Qwen GGUF"

# 搜索 SafeTensors 模型（需要转换）
csghub-lite search "Qwen"
```

模型名中带有 `-GGUF` 后缀的通常提供 GGUF 格式文件。
