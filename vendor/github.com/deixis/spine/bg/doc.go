// Package bg aims to manage all background jobs run on lego.
// This allows to keep the number of background tasks in control and drain
// them properly when it is time to stop a lego.
//
// Background jobs will typically be potentially long operations, such as
// uploading files, sending emails or push notifications. It can also
// be used for services that run infinitely like heartbeat signals or stats
// worker.
//
// Package bg guarantee that a dispatched job will be started even a registry
// is being asked to drain right after. However, there is a slim chance that
// Stop() is called before Start().
package bg
