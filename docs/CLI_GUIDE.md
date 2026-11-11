# Pulse-Vault command-line guide

The `pulse-vault` command uses the same vault files as the desktop app. It runs locally and asks for a password without echoing it when you use an interactive terminal. These examples use an installed command on your `PATH`; from a source checkout, use `.\cli.ps1` in PowerShell or `./cli.sh` in a Unix shell in place of `pulse-vault`.

See [installation](../INSTALL.md) if the command is not found. Run `pulse-vault help` to see the commands supported by your installed version.

## Create, check, and restore a vault

The commands below work in a terminal on Windows, Linux, or macOS. Change the paths for your files. Pulse-Vault prompts for the password on each command.

```text
pulse-vault create private-records
pulse-vault add private-records.pulsevault ./records ./insurance.pdf
pulse-vault list private-records.pulsevault
pulse-vault verify private-records.pulsevault
pulse-vault extract-all private-records.pulsevault ./restored
```

`create private-records` writes `private-records.pulsevault`. `add` accepts multiple files and folders in one command. When you add a folder, the folder's *contents* become entries at the vault root; paths below that folder remain intact. If two selected paths would produce the same vault name, the add stops rather than silently replacing one.

`list` shows the names and recorded sizes inside the unlocked vault. `verify` reads every stored stream and checks authentication and recorded file digests; it does not create restored files. `extract-all` writes the entries under `./restored`, creating the folder if needed. Open a restored file and compare it with your original before treating this archive as a backup. The vault does not delete the original files you added.

Use an empty restore directory for a first run. Without `--overwrite`, extraction refuses conflicting output paths. Already completed files can remain if a later file fails verification or an operation is interrupted.

## Work with one entry

Use the exact name displayed by `list`. Quote a name if it contains spaces.

```text
pulse-vault extract private-records.pulsevault "insurance.pdf" ./one-file-restore
pulse-vault delete private-records.pulsevault "insurance.pdf"
```

`extract` writes a readable copy to the chosen directory. `delete` removes an entry from the vault, not the original file on disk. Both commands ask for the password in an interactive terminal. To replace an existing entry when adding, or an existing output when extracting, use `--overwrite` deliberately after checking the destination.

To remove several entries, pass each stored path after the vault name, for example `pulse-vault delete private-records.pulsevault "old/insurance.pdf" "old/receipt.pdf"`. The command checks every name before changing anything, then rewrites the vault once for the whole batch. If a name is missing or repeated, the batch is rejected without changing the vault. The original files on disk remain untouched.

## Keep a vault current

```text
pulse-vault add private-records.pulsevault ./new-document.pdf
pulse-vault verify private-records.pulsevault
pulse-vault change-password private-records.pulsevault --profile hardened
```

Password change asks for the current and new passwords and re-encrypts the vault. It keeps the existing KDF profile unless you choose `--profile standard` or `--profile hardened`; choosing `hardened` raises the cost of offline guesses while using about 1 GiB during key derivation. A new vault can also use `create <vault> --profile hardened`. Have enough free disk space for a temporary replacement of the container. After adding, deleting, or changing the password, copy the updated `.pulsevault` file to your backup location again. An older backup may lack recent files and may still require the old password.

`pulse-vault info private-records.pulsevault` shows container information without unlocking. It does **not** verify every file. Use `verify` for that.

## Terminal session

If you prefer to enter the password once for several operations, run:

```text
pulse-vault open private-records.pulsevault
```

At the interactive prompt, enter `help` to see commands such as `list`, `add`, `extract`, `extract-all`, and `verify`. Enter `quit` to lock and leave. This console needs a real interactive terminal; if input or output is redirected, `open` displays the inventory and exits.

## Passwords in scripts

Commands run without a terminal need an explicit password source. `--password-file` reads a file containing the password; `--password-stdin` reads one line from standard input. Example:

```text
pulse-vault verify private-records.pulsevault --password-file ./vault-password.txt
```

Protect any password file with your operating system's file permissions and do not place it beside a shared vault. For a password manager or secret store that can write one line to standard input, use `--password-stdin`. The `--password <value>` option exists for compatibility, but command arguments can appear in shell history and process listings, so avoid it for normal use. Use only one password source per command.

## Command reference

| Command | Purpose |
| --- | --- |
| `create <vault>` | Make a new vault; a bare name gets `.pulsevault` appended. |
| `add <vault> <path> [more paths...]` | Add files or folder contents. |
| `list <vault>` | Show entries after unlocking. |
| `verify <vault>` | Check all encrypted streams and stored hashes. |
| `extract <vault> <name> <outdir>` | Restore one entry. |
| `extract-all <vault> <outdir>` | Restore all entries. |
| `delete <vault> <name> [more names ...]` | Remove one or more entries with one vault rewrite. |
| `change-password <vault> [--profile standard|hardened]` | Re-encrypt with a new password; preserve or deliberately change its KDF profile. |
| `info <vault>` | Show container details; exact format and file details require a password. |
| `open <vault>` | Start an interactive vault console. |
| `migrate <vault>` | Rewrite a supported legacy vault into the current format. Back it up first. |

For command syntax and the main flags, run `pulse-vault help`. The [vault format](VAULT_FORMAT.md) and [threat model](THREAT_MODEL.md) explain compatibility and the limits of integrity checks.

For measured create, incremental add, verify, extract, batch delete, and bulk path-check workloads, see the [performance explorer](https://z3r0s.github.io/Pulse-Vault/) and [benchmark method](PERFORMANCE.md).
