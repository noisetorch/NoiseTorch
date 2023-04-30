// SPDX-License-Identifier: Unlicense OR MIT

/*
Package pointer implements pointer events and operations.
A pointer is either a mouse controlled cursor or a touch
object such as a finger.

The InputOp operation is used to declare a handler ready for pointer
events. Use an event.Queue to receive events.

Types

Only events that match a specified list of types are delivered to a handler.

For example, to receive Press, Drag, and Release events (but not Move, Enter,
Leave, or Scroll):

	var ops op.Ops
	var h *Handler = ...

	pointer.InputOp{
		Tag:   h,
		Types: pointer.Press | pointer.Drag | pointer.Release,
	}.Add(ops)

Cancel events are always delivered.

Areas

The area operations are used for specifying the area where
subsequent InputOp are active.

For example, to set up a rectangular hit area:

	r := image.Rectangle{...}
	pointer.Rect(r).Add(ops)
	pointer.InputOp{Tag: h}.Add(ops)

Note that areas compound: the effective area of multiple area
operations is the intersection of the areas.

Matching events

StackOp operations and input handlers form an implicit tree.
Each stack operation is a node, and each input handler is associated
with the most recent node.

For example:

	ops := new(op.Ops)
	var stack op.StackOp
	var h1, h2 *Handler

	state := op.Save(ops)
	pointer.InputOp{Tag: h1}.Add(Ops)
	state.Load()

	state = op.Save(ops)
	pointer.InputOp{Tag: h2}.Add(ops)
	state.Load()

implies a tree of two inner nodes, each with one pointer handler.

When determining which handlers match an Event, only handlers whose
areas contain the event position are considered. The matching
proceeds as follows.

First, the foremost matching handler is included. If the handler
has pass-through enabled, this step is repeated.

Then, all matching handlers from the current node and all parent
nodes are included.

In the example above, all events will go to h2 only even though both
handlers have the same area (the entire screen).

Pass-through

The PassOp operations controls the pass-through setting. A handler's
pass-through setting is recorded along with the InputOp.

Pass-through handlers are useful for overlay widgets such as a hidden
side drawer. When the user touches the side, both the (transparent)
drawer handle and the interface below should receive pointer events.

Disambiguation

When more than one handler matches a pointer event, the event queue
follows a set of rules for distributing the event.

As long as the pointer has not received a Press event, all
matching handlers receive all events.

When a pointer is pressed, the set of matching handlers is
recorded. The set is not updated according to the pointer position
and hit areas. Rather, handlers stay in the matching set until they
no longer appear in a InputOp or when another handler in the set
grabs the pointer.

A handler can exclude all other handler from its matching sets
by setting the Grab flag in its InputOp. The Grab flag is sticky
and stays in effect until the handler no longer appears in any
matching sets.

The losing handlers are notified by a Cancel event.

For multiple grabbing handlers, the foremost handler wins.

Priorities

Handlers know their position in a matching set of a pointer through
event priorities. The Shared priority is for matching sets with
multiple handlers; the Grabbed priority indicate exclusive access.

Priorities are useful for deferred gesture matching.

Consider a scrollable list of clickable elements. When the user touches an
element, it is unknown whether the gesture is a click on the element
or a drag (scroll) of the list. While the click handler might light up
the element in anticipation of a click, the scrolling handler does not
scroll on finger movements with lower than Grabbed priority.

Should the user release the finger, the click handler registers a click.

However, if the finger moves beyond a threshold, the scrolling handler
determines that the gesture is a drag and sets its Grab flag. The
click handler receives a Cancel (removing the highlight) and further
movements for the scroll handler has priority Grabbed, scrolling the
list.
*/
package pointer
