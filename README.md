# Rendezvous server

*This is a part of the Pikonet project.*
[See the client-side.](https://github.com/mca3/pikonode)

The Pikonet Rendezvous server facilitates autoconfiguration of WireGuard nodes
to form the Pikonet meshnet.
It tracks the nodes in a network, some device information, and distributes that
information to clients which are responsible for connecting to each other.
It additionally also serves as a method for rudimentary UDP hole punching by
serving as a WireGuard peer that returns the endpoint of the client when pinged.
