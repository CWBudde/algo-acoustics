# algo-acoustics regression bundle

This archive contains the `roombench` binary and the matching scene fixtures
and event baselines from the same algo-acoustics release. Run commands from the
extracted archive root so the default relative paths resolve correctly.

On Linux or macOS:

```console
./bin/roombench run
./bin/roombench report --format markdown
```

On Windows:

```console
.\bin\roombench.exe run
.\bin\roombench.exe report --format markdown
```

Use `roombench --version` to verify the release version, source commit, and
build date. A regression binary and corpus should always be kept together;
mixing versions can report expected baseline drift as a failure.
