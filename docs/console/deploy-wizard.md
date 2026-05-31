---
title: "Model Deploy Wizard"
slug: deploy-wizard
url: /console/deploy-wizard
---
# Model Deploy Wizard

The Model Deploy Wizard deploys pre-configured or custom inference models to a Rack from the Console. It handles App creation, image import, GPU node placement, access control, and provides a built-in Playground for testing deployed models.

## When to Use

Use the Deploy Wizard when you want a guided, point-and-click path to stand up an inference model without writing a `convox.yml` by hand. It is the fastest way to launch a catalog template or a custom HuggingFace model, check GPU readiness, set access and authentication, and try the result in the Playground.

Reach for the CLI instead when you are deploying an existing application from source, scripting deployments into CI, or working with a template marked Advanced: Deploy via CLI. The wizard and the CLI manage the same Apps, so anything launched here can still be inspected and updated with `convox`.

## Prerequisites

- Rack version **3.24.6** or later (V3 only; V2 Racks are not supported)
- The `nvidia_device_plugin_enable` Rack parameter set to `true`
- GPU nodes available via Karpenter, custom Karpenter NodePools, or EKS Managed Node Groups
- Node disk size of at least 100 GB for GPU model images (set `karpenter_node_disk` or `node_disk`)

## Accessing the Wizard

Navigate to **Organization > Rack > Deploy Model** in the Console.

## Step 1: Select a Template

### GPU Readiness Check

Before displaying templates, the wizard verifies your Rack's GPU readiness:

- NVIDIA device plugin enabled
- At least one GPU node provisioning method configured (Karpenter with GPU families, custom GPU NodePool, or EKS Managed Node Group with a GPU instance type)
- Adequate node disk size

If checks fail, the wizard displays guided setup commands for three provisioning options:

1. **Karpenter (recommended):** Adds GPU instance families to the default Karpenter pool for automatic GPU node provisioning.
2. **Karpenter Custom GPU NodePool:** Creates a dedicated NodePool isolated from general workloads.
3. **Additional Node Groups (EKS Managed):** Provisions fixed GPU capacity via EKS managed node groups.

### Inference Catalog

The catalog contains pre-configured templates across seven categories:

| Category | Description |
|---|---|
| LLM Serving | Text generation models (Llama, Mistral, Qwen, DeepSeek, Phi, Gemma) |
| Speech | Speech recognition and text-to-speech (Whisper, Kokoro, Orpheus) |
| Image Generation | Image generation and editing (Stable Diffusion, FLUX, ComfyUI) |
| Video Generation | Video synthesis (LTX-Video, CogVideoX, Wan) |
| RAG Pipeline | Retrieval-augmented generation stacks |
| Embedding | Text embedding models |
| Dev/Prototyping | General-purpose inference servers (Ollama) |

Filter templates by category or search by name, description, or engine. Featured templates are sorted to the top.

Each template card shows GPU requirements, the serving engine (vLLM, SGLang, TGI, TEI, Speaches, ComfyUI, Ollama), difficulty level, and whether it exposes an OpenAI-compatible API.

### CLI-Only Templates

Some templates require a source build and cannot be deployed directly from the Console. These appear in an "Advanced: Deploy via CLI" section with clone and deploy commands.

### Custom Model Deployment

Select "Deploy Custom Model" to deploy any HuggingFace model. Paste a model ID or full HuggingFace URL, and the wizard auto-detects:

- Model architecture and parameter count
- Serving framework (vLLM, SGLang, TGI, TEI, or Speaches)
- GPU type and count based on estimated VRAM requirements
- Whether the model is gated (requires license acceptance on HuggingFace)

Advanced options allow overriding the detected framework, GPU type, GPU count, quantization (AWQ, GPTQ, FP8), extra CLI arguments, and service port.

## Step 2: Configure

### App Name and GPU Placement

Enter an App name (lowercase letters, numbers, hyphens; max 63 characters). The wizard displays GPU requirements from the template and an estimated monthly cost.

If multiple GPU node targets are available (Karpenter default pool, custom NodePools, EKS node groups), select which target to run the model on. The wizard validates that the selected target meets the template's GPU and VRAM requirements.

### Environment Variables

Templates that require credentials (e.g., `HUGGING_FACE_HUB_TOKEN`) prompt for them here. Token validation runs on blur to verify the token is valid before deployment.

### Access and Security

Configure how the deployed model is accessed:

- **Private (default):** Internal to the Rack network. Accessible from the Console Playground and from other Services via `https://<service>.<app>.<rack>.local`. To access from your local machine, use `convox proxy` (shown after deployment completes).
- **Public:** Internet-accessible endpoint. For frameworks that support it (vLLM, SGLang, TGI, TEI, Speaches), configure API key authentication with auto-generated or custom keys. Frameworks without built-in auth display a warning requiring explicit acknowledgment.

## Step 3: Deploy

The wizard executes three steps:

1. **Creating application:** Creates the App on the Rack.
2. **Importing image and promoting Release:** Imports the container image and promotes a Release with the generated `convox.yml`.
3. **Model starting up:** Waits for the model to pass health checks.

After deployment completes, the page splits into two panels:

- **Left panel:** Deployment summary, access configuration, View App link, deployment logs, and CLI access instructions for internal Services (`convox proxy` command).
- **Right panel:** Built-in Model Playground.

### Session Persistence

The wizard saves deployment sessions to localStorage. If you navigate away and return, a banner offers to resume the previous session.

## Model Playground

The Playground auto-detects the deployed model's API format and presents the appropriate interface:

| Format | Interface | Use case |
|---|---|---|
| Chat | Conversational UI with message history | LLM text generation |
| Audio | Upload and transcribe audio files | Speech recognition |
| TTS | Text input with voice selection and audio playback | Text-to-speech |
| Image | Text prompt with generated image display | Image generation |
| Video | Text/image prompt with video playback | Video generation |
| Raw | Manual HTTP request builder | Any API |

For public Services with API key authentication, the Playground forwards the key automatically.

The Playground is also available on each Service's detail page under the "Test Model" tab, independent of the Model Deploy Wizard.

## See Also

- [Workload Placement](/configuration/scaling/workload-placement)
- [Manifest GPU Configuration](/reference/primitives/app/service#scalegpu)
- [GPU Dashboard](/console/gpu-dashboard)
- [Budget Management](/console/budget-management)
