#!/usr/bin/env python3
"""Drive an interactive containerlab shell through a PTY and check terminal restoration."""

import pathlib
import sys

import pexpect


WRAP_SESSION = """\
prefix="$1"
shift
stty -g > "$prefix.before" || exit 96
"$@"
status=$?
stty -g > "$prefix.after" || exit 96
exit "$status"
"""


def main() -> int:
    if len(sys.argv) < 3:
        raise SystemExit(f"usage: {sys.argv[0]} STATE_PREFIX COMMAND [ARG ...]")

    state_prefix = pathlib.Path(sys.argv[1])
    child = pexpect.spawn(
        "/bin/sh",
        ["-c", WRAP_SESSION, "sh", str(state_prefix), *sys.argv[2:]],
        encoding="utf-8",
        timeout=30,
    )
    child.logfile = sys.stdout

    try:
        child.expect_exact("/ # ")
        child.sendline("printf 'interactive-%s\\n' \"$((6 * 7))\"")
        child.expect_exact("interactive-42")
        child.sendline("exit 7")
        child.expect(pexpect.EOF)
    finally:
        child.close(force=True)

    if child.exitstatus != 7:
        return child.exitstatus or 1

    before = state_prefix.with_suffix(".before").read_bytes()
    after = state_prefix.with_suffix(".after").read_bytes()
    if before != after:
        print(f"terminal was not restored: {before!r} != {after!r}")
        return 98

    return 7


if __name__ == "__main__":
    raise SystemExit(main())
