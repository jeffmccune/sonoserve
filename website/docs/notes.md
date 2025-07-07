# Notes

Session one started to get a bit off the rails around turn 30.  Claude kept losing the thread trying to make changes when it wasn't in an effective edit, run, test, kill, repeat loop.

I decided to start a new session instead of continue the old one.  The new session 2 starts off trying to figure out how to get the play endpoint to queue and play the mp3 file embedded into the go server.  The error returned from the speaker indicates the mime time is wrong.

## Speaker Name Apostrophe Issue
The speaker named "Kid's Room" uses a different Unicode apostrophe character than the standard ASCII apostrophe. This causes Claude to munge the character incorrectly when processing, which results in the play endpoint not finding the speaker in the cache. To avoid this issue, the speaker will be manually renamed in the Sonos app to use a name without special characters.
