# SRC-PATH-TYPE recipe (not committed special inodes)

Git cannot reliably store FIFO, socket, or device nodes. This directory **locks construction + expected `rule_id`**, not a checked-in FIFO.

```bash
node setup.mjs
```

Writes a throwaway tree under `work/` (gitignored). It does **not** run `a2h register`. Contract Freeze expected: every constructed non-regular or symlink candidate fails **SRC-PATH-TYPE**.

| Case | Construct | Expected |
| --- | --- | --- |
| symlink leaf | `ln -s` | fail `SRC-PATH-TYPE` |
| symlink component | `ln -s` on a directory component | fail `SRC-PATH-TYPE` |
| directory | `mkdir` at the declared path | fail `SRC-PATH-TYPE` |
| FIFO | `mkfifo` (POSIX; skip Windows) | fail `SRC-PATH-TYPE` |
| Unix socket | bind under `os.tmpdir()/amx-pt/<pid>/` (Darwin `sun_path` ~104 bytes; skip Windows) | fail `SRC-PATH-TYPE` |
| device | synthetic `stat` metadata only; **do not** `mknod` | fail `SRC-PATH-TYPE` |

`source-template/` is an otherwise legal minimal System. `setup.mjs` copies it and rewrites `sop` per case.
