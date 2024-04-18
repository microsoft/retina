package ktime

// MonotonicOffset is the calculated skew between the kernel's
// monotonic timer and UTC.
//
// If clock readings were instantaneous, this would mean that
// MonotonicTimer - MonotonicOFfset = the UTC Boot Time, but
// that is idealized and there will be some small errror.
var MonotonicOffset = calculateMonotonicOffset()
