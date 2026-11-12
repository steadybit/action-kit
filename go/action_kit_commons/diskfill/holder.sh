#!/bin/sh
# Usage: holder.sh FILE SECONDS FILL_COMMAND...
#
# Creates FILE, unlinks it right away and keeps it open on fd 3 while
# FILL_COMMAND writes to it. The kernel frees the space once the last
# descriptor is closed, so nothing is left behind when the extension is killed
# (BusinessMap card 26837).
#
# Ends on SIGINT/SIGTERM, after SECONDS (even while the fill still runs) or on
# EOF on stdin. Stdin is a pipe only the extension writes to; runc forwards it
# into the sidecar and closes it when runc dies, crun passes it on as is.
#
# Children are SIGKILLed and reaped before the holder exits, as the runtime
# considers the sidecar stopped once the holder is gone. SIGKILL, because a
# child signalled between fork and exec runs the shell's trap and loses the
# signal. Hence every external command is a direct child of the holder.
# Everything runs in the background, as traps are deferred while a foreground
# command runs.

fill='' timer='' lifeline='' stopped=''
trap 'trap "" INT TERM; kill -KILL $fill $timer $lifeline 2>/dev/null; wait' EXIT
file=$1
seconds=$2
shift 2

# The watcher runs builtins only and notices the end of the sleep when its
# write fails. INT and TERM are ignored meanwhile: bash waits for the first
# command of a pipeline when a trap runs while it sets up the pipeline.
trap '' INT TERM
# shellcheck disable=SC2216
{ trap '' PIPE; while echo; do :; done; kill -TERM $$; } 2>/dev/null | sleep "$seconds" &
timer=$!

# Until all pids are known a signal only sets stopped, as a trap between
# "cmd &" and "pid=$!" would miss that child.
trap 'stopped=1' INT TERM
exec 3>"$file" || exit 1
rm -f "$file" || exit 1

# Background jobs get /dev/null as stdin, hence the copy to fd 4. Only the
# fill may inherit fd 3, or the space outlives the holder.
exec 4<&0
{ while read -r _; do :; done; kill -TERM $$; } <&4 3>&- 4<&- &
lifeline=$!
exec 4<&-

"$@" &
fill=$!

trap 'exit 0' INT TERM
[ -z "$stopped" ] || exit 0
wait $fill || exit
wait $timer
