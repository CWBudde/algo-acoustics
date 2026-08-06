#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
repo_dir="$(cd "$script_dir/.." && pwd)"
output_dir="$repo_dir/web/audio"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

for command in espeak-ng ffmpeg sox; do
	if ! command -v "$command" >/dev/null; then
		echo "missing required command: $command" >&2
		exit 1
	fi
done

mkdir -p "$output_dir"

espeak-ng \
	-w "$work_dir/speech-source.wav" \
	-s 150 \
	-p 42 \
	-a 135 \
	"Hear how the shape and surfaces of a room transform this sound."
sox "$work_dir/speech-source.wav" -r 48000 -c 1 "$work_dir/speech.wav" \
	gain -6 highpass 90 lowpass 10000 fade h 0.02 0 0.08 gain -n -2

ffmpeg -hide_banner -loglevel error -y \
	-f lavfi -i "anoisesrc=color=pink:sample_rate=48000:duration=0.8:seed=1905" \
	-af "highpass=f=650,lowpass=f=12000,volume='0.95*exp(-14*t)':eval=frame,afade=t=out:st=0.5:d=0.3" \
	"$work_dir/clap.wav"

notes=(
	"C4 E4 G4 B4"
	"A3 C4 E4 G4"
	"F3 A3 C4 E4"
	"G3 B3 D4 A4"
)
for index in "${!notes[@]}"; do
	read -r note1 note2 note3 note4 <<<"${notes[$index]}"
	sox -n -r 48000 -c 1 "$work_dir/chord-$index.wav" \
		synth 1.2 sine "$note1" sine "$note2" sine "$note3" sine "$note4" \
		remix - gain -18 fade h 0.02 1.2 0.18
done
sox "$work_dir/chord-0.wav" "$work_dir/chord-1.wav" \
	"$work_dir/chord-2.wav" "$work_dir/chord-3.wav" "$work_dir/music.wav" \
	gain -n -2

for name in clap speech music; do
	ffmpeg -hide_banner -loglevel error -y \
		-i "$work_dir/$name.wav" \
		-ar 48000 -ac 1 -codec:a libmp3lame -b:a 64k \
		-map_metadata -1 "$output_dir/$name.mp3"
done

echo "Generated demo audio in $output_dir"
