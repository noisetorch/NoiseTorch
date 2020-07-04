// SPDX-License-Identifier: Unlicense OR MIT

/*
Package clip provides operations for clipping paint operations.
Drawing outside the current clip area is ignored.

The current clip is initially the infinite set. An Op sets the clip
to the intersection of the current clip and the clip area it
represents. If you need to reset the current clip to its value
before applying an Op, use op.StackOp.

General clipping areas are constructed with Path. Simpler special
cases such as rectangular clip areas also exist as convenient
constructors.
*/
package clip
