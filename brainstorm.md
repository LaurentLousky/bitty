
stage 1

usage
--
./stream [magnet link here]
- vlc opens and begins to stream video
- user can see streaming rate in terminal


stage 2

usage
--
raspberry pi running bittorrent client and web server, hooked up to tv for display
- go to local website on phone to interact with client
- browse torrents
- select torrent
- vlc opens and begins to stream video




P2P thoughts
--

each peer upon hadndshake can immediately send me a bitfield.
    should i close the connection w/ them if they dont send me a bitfield?
    or
    should i keep them around and send them req messages and get back have messages?