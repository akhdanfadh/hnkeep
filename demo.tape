Output hnkeep-demo.gif

Require make
Require tr
Require wc
Require awk
Require hnkeep
Require jq

Set Shell "bash"
Set FontSize 18
Set Padding 24
Set Width 1000
Set Height 600
Set CursorBlink false

Hide Type "hnkeep -v" Enter Show
Sleep 1s

Enter
Sleep 500ms
Type "hnkeep -i demo-input.txt -dry-run" Sleep 1s Enter
Sleep 3s

Type "clear" Sleep 500ms Enter

Sleep 500ms
Type "hnkeep -i demo-input.txt \" Sleep 200ms Enter
Type "-t 'hnkeep:test' -note-template '{{author}}' \" Sleep 200ms Enter
Type "-after 2024-01-01 -before 2025-01-01 \" Sleep 200ms Enter
Type "| jq '.bookmarks[10]'" Sleep 2s Enter
Sleep 8s

Type "clear" Sleep 500ms Enter

Sleep 500ms
Env KARAKEEP_API_URL "http://localhost:3000/api/v1"
Env KARAKEEP_API_KEY "ak2_XXXXXXXXXXXXXXXXXXXXXXXX"
Type "make sample-input i=demo-input.txt n=100 \" Sleep 200ms Enter
Type "| hnkeep -sync -t 'hnkeep:test-sync' \" Sleep 200ms Enter
Type "-api-url ${KARAKEEP_API_URL} \" Sleep 200ms Enter
Type "-api-key ${KARAKEEP_API_KEY}" Sleep 2s Enter
Wait+Screen /Sync results/
Sleep 8s
