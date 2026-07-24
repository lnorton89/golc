// Package ipc implements build-selected, owner-only local transport
// connecting the Art-Net daemon (internal/artnet) to its CLI and UI clients:
// a Windows named pipe or a Unix domain socket. Dial and Forward form the
// client side; a Listener and dispatch loop form the daemon side. Request,
// Result, and Handler are field-for-field identical
// in shape to internal/command's own types, but declared independently
// here rather than imported: internal/command imports this package
// directly to build its "artnet ..." client routes, so importing
// internal/command back from here (or from internal/artnet) would close a
// hard import cycle. A subsystem package never imports internal/command;
// only internal/command imports the subsystem.
package ipc
