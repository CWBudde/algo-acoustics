# Demo audio

These project-owned clips were created specifically for the browser demo and
are distributed under the repository's MIT license:

- `clap.mp3` is a shaped pink-noise transient.
- `speech.mp3` reads an original sentence using eSpeak NG.
- `music.mp3` is an original four-chord synthesized music bed.

All files are mono, 48 kHz, 64 kbit/s MP3. They are committed so building and
serving the demo has no audio-tool dependencies. To regenerate them on a system
with eSpeak NG, FFmpeg, and SoX installed, run:

```bash
bash scripts/generate-demo-audio.sh
```
