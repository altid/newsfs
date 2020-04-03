# newsfs

newfs is a very simple Altid service for aggregating RSS + ATOM feeds.

Feed urls are read, line by line from the feeds file (-conf to set this), and newsfs will check each in turn for new items.
After going through the list, newsfs will sleep for a timeout.
