
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

plan for now
------------

flow
----

1) first go through each peer sequentially until we get the metadata
2) create an input channel that has a queue of pieces that need to be downloaded
   create an output channel that has all the downloaded pieces   
3) start a goroutine for each peer connection
4) each connection will:
    1. handshake, accept bitfield, accept have messages, etc
    2. send an interested message
    3. once unchoked, begins taking pieces from the input channel queue
       and attempting to download them
    4. if it fails to download them, it places it back on the queue (that way another go-routine can pick it up and attempt to download it)
