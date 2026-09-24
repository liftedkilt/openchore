# AI features

Everything here is optional and off by default. OpenChore works fully without
it: chores are still read aloud by the device's own voice, and photo proof is
still checked by a parent.

| Feature | Needs | What it does |
|---------|-------|--------------|
| Photo review | a vision model | Adds a note to photo chores waiting in **Approvals** ("Bed is made, pillows on the floor"). Can approve clear passes for you. |
| Weekly summaries | a text model | Writes up each person's week. Shown in Reports, and each Monday shared via Discord and the `report.weekly_summary` webhook. |
| Description drafts | a text model | The **AI** button next to Description in the new-chore wizard writes a short, kid-friendly description. |
| Recorded voices | a speech service | Records each chore's title and description in a natural voice, instead of the browser's built-in one. |

## The AI only advises

Photo review never rejects a chore and never keeps a kid waiting. The kid
finishes the chore, it goes to **Approvals** like any chore that needs
approval, and the review runs in the background. When it's done, its note
appears on the chore's card in Approvals ("AI: looks done · 92% sure", then
the model's note) and you decide. Kids never see it.

If you turn on **Approve clear passes automatically**, a chore the model is
confident is done (at or above the threshold you set) is approved for you,
through the same path as tapping Approve. Anything else waits for you.

Photo review applies to chores that **need approval**. A photo on a chore that
doesn't need approval is just kept as a record.

Kids can also finish a photo chore without a photo ("No photo? Finish
anyway…"); it then waits for a parent instead.

## Connecting a model

OpenChore talks to anything that speaks the OpenAI API: a local server or a
hosted provider. Set these on the API container (for Docker Compose, in `.env`
next to `compose.yaml`):

| Variable | Example | |
|----------|---------|---|
| `AI_BASE_URL` | `http://llama:8080/v1` | Base URL **including** `/v1` (or the provider's equivalent). AI features are off when unset. |
| `AI_MODEL` | `gemma-4-e4b` | Model name as the server knows it. Required with `AI_BASE_URL`. |
| `AI_API_KEY` | `sk-…` | Sent as a bearer token. Leave empty for local servers. |
| `TTS_BASE_URL` | `http://kokoro:8880/v1` | Speech service base URL. Recorded voices are off when unset. |
| `TTS_MODEL` | `kokoro` | Defaults to `kokoro`. |
| `TTS_API_KEY` | | For hosted speech services. |

Restart the API after changing them. **Manage → Settings → AI** shows what's
connected and has a **Try photo review** box for testing a model on your own
photos before you rely on it.

### Local: the bundled compose profiles

```bash
# .env
AI_BASE_URL=http://llama:8080/v1
AI_MODEL=gemma-4-e4b
TTS_BASE_URL=http://kokoro:8880/v1
```

```bash
docker compose --profile ai --profile tts up -d
```

- `ai` runs llama.cpp's server with **Gemma 4 E4B** (Q4). It downloads the
  model (about 5 GB) on first start and needs about 6 GB of RAM. A photo review
  takes a few seconds to a minute depending on your CPU.
- `tts` runs **Kokoro** (about 2 GB of RAM).

Either profile can run on its own.

### Local: Ollama, LM Studio, or your own server

Point `AI_BASE_URL` at it. For Ollama that's `http://<host>:11434/v1`, with
a model you've pulled, for example `AI_MODEL=gemma4:e4b` or
`AI_MODEL=qwen3.5:9b`.

### Hosted

Use the provider's OpenAI-compatible endpoint, a vision-capable model, and
`AI_API_KEY`. At household volumes a small hosted model costs very little: a
photo review is a fraction of a cent. **Photos of your home and your kids are
sent to that provider**, so decide if you're comfortable with that. The local
options keep everything on your machine.

### Choosing a model

- **Smallest footprint:** Gemma 4 E4B (the bundled default). Good at "is this
  bed made"; less reliable on subtle judgements.
- **Better judgement, more RAM:** a 9B-class vision model such as Qwen 3.5 9B
  (roughly 8–10 GB).
- **No hardware to spare:** a small hosted model.

Structured output (JSON schema) is requested for photo reviews. Servers that
don't support it still work; the reply is parsed leniently.

## Settings

Under **Manage → Settings → AI** (or `ai:` in `config.yaml` on first boot):

| Setting | Default | |
|---------|---------|---|
| Review photos awaiting approval (`ai_photo_review`) | off | |
| Approve clear passes automatically (`ai_auto_approve`) | off | |
| Auto-approve confidence (`ai_auto_approve_threshold`) | 0.85 | |
| Write a weekly summary (`ai_weekly_summary`) | off | Generated on Monday after noon for everyone with activity last week. |
| Read-aloud voice (`tts_voice`) | `af_heart` | A voice your speech service knows (`alloy` etc. for OpenAI). Changing it re-records every chore. |

Read-aloud audio is recorded when a chore is created, and again when its title
or description changes. Chores without audio are filled in at startup.

## Upgrading from the LiteRT/Ollama setup

This release replaces the bundled AI sidecar with the OpenAI-compatible setup
above.

- `AI_ENDPOINT`, `OLLAMA_ENDPOINT` and `TTS_ENDPOINT` are no longer read (the
  server logs a warning if they're set). Use `AI_BASE_URL` + `AI_MODEL` and
  `TTS_BASE_URL`, **with** the `/v1` suffix.
- The `litert` image is no longer built, and the `litert` and `ollama` compose
  services are gone. Run `docker compose up -d --remove-orphans` to clean up
  the old containers, and remove the old `litert-model` / `ollama-models`
  volumes (see `docker volume ls`) if you don't need them.
- Kokoro moved from the `ai` profile to its own `tts` profile.
- Photo review no longer rejects chores. Past AI rejections (`ai_rejected`)
  are removed by the migration; they never earned points.
- Your "AI photo review" switch carries over as **Review photos awaiting
  approval**; auto-approval starts off.
- The rewritten "spoken description" is gone: audio now reads the chore's own
  title and description, and is re-recorded on first start if a speech
  service is configured.
- Point and time suggestions have been removed.
