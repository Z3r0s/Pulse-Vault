# Measuring Pulse-Vault

The CLI benchmark exercises the same vault implementation used by the desktop app. It creates disposable synthetic files in a temporary directory, builds a vault, adds the folder, verifies it, restores it, and checks the SHA-256 digest and path of every restored file. It removes the temporary data when finished. It does not measure GUI rendering or startup.

From the repository root, build the CLI and run the benchmark:

```sh
go build -o pulse-vault ./gui-go/cmd/pulse-vault
python scripts/benchmark-cli.py ./pulse-vault --files 1000 --bytes-per-file 4096
python scripts/benchmark-cli.py ./pulse-vault --files 8 --bytes-per-file 10485760 --content random
```

On Windows, build `pulse-vault.exe` and pass that filename instead. Output is JSON with operation times, input and vault sizes, and the number of verified files. If the optional `psutil` Python package is installed, it also samples each CLI process's resident memory every 20 ms; otherwise memory values are `null`. Sampling can miss short-lived peaks and does not include other processes.

The default `standard` password profile reflects normal use. `--profile fast` is for disposable tests only: it enables the deliberately weaker test-only KDF and must never be used for real vaults. Repeated content compresses unusually well; `--content random` provides a less compressible comparison. Disk cache, storage, CPU load, file count, compression, KDF settings, and operating system all affect results. Compare runs on the same machine and workload; don't treat them as universal speed claims.
