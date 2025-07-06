# sonoserve

Simple server for my 5 year old to control his Sonos Play:1 speaker using an
M5Stack CardPuter v1.1 esp32s3 device.

As a kid, I'd like to restart my favorite playlist so that I can listen to my favorite music.

As a kid, I'd like to play my favorite tracks.

The plan to implement these user stories is to put most of the logic in a Go executable and use the cardputer as a simple hotkey controller.  Ideally he will be able to turn it on, wait for a green light, then push one button to do the thing he wants to do.
