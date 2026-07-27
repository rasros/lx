#!/usr/bin/env bash
# Regenerates the media fixtures. ffmpeg is a development tool here, not a
# dependency of lx: the fixtures are committed so the tests never invoke it.
#
# The point of using a real encoder is that its output carries the padding,
# edit lists and seek tables that a hand-built fixture would not, so the
# parsers are exercised against files of the shape they will actually meet.
#
# Usage: cd cmd/lx/testdata/media && ./gen_fixtures.sh
set -euo pipefail

ff() { ffmpeg -v error -y "$@"; }

VIDEO=(-f lavfi -i "testsrc=s=32x32:d=1:r=10")
AUDIO=(-f lavfi -i "sine=f=440:d=1:r=8000")

# Video and audio muxed together, the common case for a video file.
ff "${VIDEO[@]}" "${AUDIO[@]}" -c:v libx264 -pix_fmt yuv420p -c:a aac -b:a 32k sample.mp4

# Video with no audio track, so the absence of an Audio line is covered.
ff "${VIDEO[@]}" -c:v libx264 -pix_fmt yuv420p silent.mp4

# QuickTime, which shares the box layout but not the extension.
ff -i sample.mp4 -c copy sample.mov

# Audio-only containers.
ff "${AUDIO[@]}" -c:a aac -b:a 32k sample.m4a
ff "${AUDIO[@]}" -c:a pcm_s16le sample.wav
ff "${AUDIO[@]}" -c:a flac sample.flac

# A header that stops mid-box, to pin the fall back to the container alone.
head -c 32 sample.mp4 > truncated.mp4

ls -l ./*.mp4 ./*.mov ./*.m4a ./*.wav ./*.flac
