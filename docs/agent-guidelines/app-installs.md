# User-Scope App Installs

- AI app installers should default to the current user's writable directories,
  such as `~/.local/bin`, `~/.local/share/<app>`, or platform equivalents.
- Do not use system-wide install locations or commands that normally require
  root/Admin privileges unless the app cannot work without them.
- Do not require users to run `sudo csghub-lite ...` for AI app installation.
  Running as root changes HOME/config/server ownership and can break the normal
  user workflow.
- If elevated permissions are unavoidable, explain why in code comments or
  user-facing logs, and keep the privileged step as narrow as possible.
- For npm-based apps, prefer a user-owned `--prefix` plus a launcher in the
  user's bin directory over `npm install -g` to the default global prefix.
- Update uninstallers to remove only the user-owned files that the installer
  created.
