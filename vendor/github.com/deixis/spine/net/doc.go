// Package net contains the main logic to send and receive requests
//
// I defines interfaces shared by other packages that accept or send requests
// Packages that implement these interfaces include:
//  * net/cache
//  * net/grpc
//  * net/http
//
// It also manages the lifecycle of the handlers. All handlers should be
// registered to this package in order to be gracefuly stopped (drained)
// when the application shuts down.
package net
