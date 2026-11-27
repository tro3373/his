# TODO

- test
- actions
- release


# DONE

# Open the month logs

`his` with no arguments lists the month logs of every log under `gen.logs` in fzf, newest month first, and opens the chosen ones (Tab for several) in `$EDITOR` (default `nvim`).
The preview shows the end of the file.
When stdout is not a terminal, it prints the path of every month log, one per line.
The time summary moved to `his latest` (alias `his show`) and `his tag`.

```yaml
gen:
  logs:
    - name: job
      dir: ~/notes/job      # <dir>/<YYYY>/<YYYY-MM-DD>-log.md
    - name: prv
      dir: ~/notes/prv
```

`his` reads only `gen.logs`. `his gen --help` lists the rest of `gen`.

# Activity log

Work done only in a shell or nvim leaves no Claude Code session, so `his gen` would read it as a break.
`his act` appends one line to the Activity log: the time, the working directory and its git branch.
The command being run is never written.
`his gen` reads the log along with the sessions.

## zsh

```zsh
eval "$(his hook zsh)"
```

Add it to `~/.zshrc`.
It runs `his act` in the background on every prompt, command and directory change.
Sourcing it twice registers it once.

## Neovim

```lua
vim.api.nvim_create_autocmd({ "DirChanged", "CursorHold", "BufWritePost", "FocusGained" }, {
  callback = function()
    vim.system({ "his", "act" }, { cwd = vim.fn.getcwd(), detach = true })
  end,
})
```

## Config

`gen.activity` in `~/.his`. `his act` reads only this key, so it works with the rest of `gen` absent.

```yaml
gen:
  activity:
    dir: ~/.local/state/his/activity  # one YYYYMMDD.tsv per day
    retention_days: 35                # at least gen.catch_up_days; older files are deleted by `his gen`
    ignore: ["~"]                     # working directories left out of the log, matched exactly
```

`ignore` matches a working directory exactly, so `~` leaves out the home directory itself and not what is under it.
It applies to the Activity log only, never to Claude Code sessions.
