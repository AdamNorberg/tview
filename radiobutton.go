package tview

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// RadioButton is a Checkbox that is associated with exactly one
// RadioButtonGroup. A RadioButtonGroup allows at most one of its boxes to
// be checked at any time; if another box is checked, the original box is
// unchecked. Do not instantiate a RadioButton directly; it must be
// instantiated from the RadioButtonGroup it should be associated with.
//
// A RadioButton must not be copied, or its associated RadioButtonGroup will
// feel sad and confused.
type RadioButton struct {
	// The underlying Checkbox being gathered into a RadioButtonGroup. Do not
	// invoke SetChangedFunc on this field directly; doing so will break the
	// RadioButtonGroup.
	*Checkbox

	// RadioButton needs to use the Changed callback from Checkbox itself, so
	// store the user's callback here and provide it when done.
	//
	// This callback is invoked *before* the Changed callback for the previously
	// checked field (if any), but *after* RadioButton has updated which buttons
	// it reports to be checked.
	//
	// If this callback changes button states, the result is undefined.
	changed func(checked bool)

	parent    *RadioButtonGroup
	parentIdx int
}

// SetChangedFunc sets a handler which is called when the checked state of
// this radio button is changed. The handler function receives the new state.
//
// When a RadioButton is automatically unchecked because another RadioButton
// in its group became checked, this is invoked after the callback for the
// button that just became checked.
//
// In a RadioButtonGroup configured to "stay checked", an attempt to uncheck
// this button directly (rather than by checking anything else) will *not*
// invoke this handler.
func (r *RadioButton) SetChangedFunc(handler func(checked bool)) *RadioButton {
	r.changed = handler
	return r
}

// Index returns the index the parent RadioButtonGroup uses to identify this
// RadioButton.
func (r *RadioButton) Index() int {
	return r.parentIdx
}

// Parent returns the RadioButtonGroup this RadioButton is associated with.
func (r *RadioButton) Parent() *RadioButtonGroup {
	return r.parent
}

// RadioButtonGroup associates a set of radio buttons with each other. Exactly
// one radio button in a group is allowed to be checked at once.
//
// RadioButtonGroup is not a drawable widget.
type RadioButtonGroup struct {
	// Children managed by this RadioButtonGroup.
	buttons []*RadioButton

	// Defaults to use when constructing new RadioButton instances. If these are
	// tcell.ColorDefault, this sticks with the defaults provided in
	// NewCheckbox.
	labelColor, fieldBackgroundColor, fieldTextColor tcell.Color

	// Default to use when constructing new RadioButton instances. If this is
	// "", this sticks with the default in NewCheckbox. Note that the default
	// for this field itself is "o", emulating the circular shape of radio
	// buttons in popular operating systems.
	checkedString string

	// Which button (if any) is currently checked? A negative value indicates
	// that nothing was previously checked.
	checkedIdx int
	// Which cell (if any) was last checked before this one? Applications can
	// use this if they want to handle transitions between different pairs of
	// states in some specific way. Note that if the active state is "nothing
	// is checeked", then an item becomes checked, prevActiveIdx is indeed
	// overwritten with -1, indicating the last two steps of the path from
	// original -> nothing -> final. But, see "stayChecked" for groups that
	// want to avoid this state entirely, which is the default behavior.
	prevCheckedIdx int

	// If stayChecked is true, then an attempt to uncheck the single active
	// box in the group is immediately reverted (before any draw operation has
	// an opportunity to fire) without invoking user-provided callbacks.
	stayChecked bool

	// RadioButtonGroup needs to interpret "checked" events incoming from its
	// children in different ways depending on whether it is currently in the
	// middle of doing something else.
	ignoreEvent bool

	// User-provided callback.
	hasFinishedChanging func(*RadioButton, *RadioButton)
}

// NewRadioButtonGroup creates a new RadioButtonGroup with no buttons. `o` is
// the default symbol for a checked RadioButton. By default, a RadioButtonGroup
// will not allow its elements to be unchecked directly; they can only be
// unchecked by checking a different element in the group. Call
// `SetStayChecked(false)` to allow zero items to be checked once any item
// has been checked for the first time.
func NewRadioButtonGroup() *RadioButtonGroup {
	return &RadioButtonGroup{
		checkedString:  "o",
		checkedIdx:     -1,
		prevCheckedIdx: -1,
		stayChecked:    true,
	}
}

// NewRadioButton returns a new RadioButton assigned to this group.
//
// RadioButtons cannot be removed from the group once they are created.
// However, a radio button group is not itself a drawable component,
// so it can be removed from the UI.
func (g *RadioButtonGroup) NewRadioButton() *RadioButton {
	c := NewCheckbox()
	if g.checkedString != "" {
		c.SetCheckedString(g.checkedString)
	}
	if g.labelColor != tcell.ColorDefault {
		c.SetLabelColor(g.labelColor)
	}
	if g.fieldBackgroundColor != tcell.ColorDefault {
		c.SetFieldBackgroundColor(g.fieldBackgroundColor)
	}
	if g.fieldTextColor != tcell.ColorDefault {
		c.SetFieldTextColor(g.fieldTextColor)
	}

	i := len(g.buttons)
	c.SetChangedFunc(func(checked bool) {
		g.handleCheckEvent(i, checked)
	})

	r := &RadioButton{
		Checkbox:  c,
		parent:    g,
		parentIdx: i,
	}

	g.buttons = append(g.buttons, r)
	return r
}

// SetStayChecked configures whether this group prevents the selected
// RadioButton from becoming unchecked by means other than checking a new one.
// This includes direct calls to the SetChecked functions on those
// RadioButton instances themselves, and providing a negative index to
// Check, as well as user input.
//
// User-provided Changed callbacks will not be invoked if an attempted
// unchecking is reverted.
func (g *RadioButtonGroup) SetStayChecked(stayChecked bool) {
	g.stayChecked = stayChecked
}

// Len returns the number of RadioButton instances associated with this
// RadioButtonGroup.
func (g *RadioButtonGroup) Len() int {
	return len(g.buttons)
}

// Get returns the RadioButton at the provided index. If the provided index
// is invalid, this panics.
func (g *RadioButtonGroup) Get(idx int) *RadioButton {
	return g.buttons[idx]
}

// Checked returns the currently-checked RadioButton in the group, if any.
func (g *RadioButtonGroup) Checked() *RadioButton {
	if g.checkedIdx < 0 {
		return nil
	}
	return g.buttons[g.checkedIdx]
}

// PreviouslyChecked returns the RadioButton in the group that was checked
// immediately before the RadioButtonGroup entered its current state.
//
// If the previous state was "unchecked", either because the group has never
// had any RadioButton instances checked at all, the current checked RadioButton
// is the first that has ever been checked in the group, or becuase the user
// explicitly unset a checked item (while the RadioButtonGroup is not in
// "stay checked" mode) before checking the curent one, this returns nil.
func (g *RadioButtonGroup) PreviouslyChecked() *RadioButton {
	if g.prevCheckedIdx < 0 {
		return nil
	}
	return g.buttons[g.prevCheckedIdx]
}

// Check sets the specified child (identified by index) to be the checked
// item in the group. To uncheck everything in the group, provide a negative
// index; note that a RadioButtonGroup in "stay-checked" mode (which is the
// default mode; see `SetStayChecked` for more) will ignore this.
//
// If you have the *RadioButton you want to check, call its SetChecked
// method directly instead of invoking this with its Index.
//
// If idx is out of range in the positive direction, Check panics.
func (g *RadioButtonGroup) Check(idx int) {
	if idx < 0 {
		if g.stayChecked || g.checkedIdx < 0 {
			// Nothing to do.
			return
		}
		g.buttons[g.checkedIdx].SetChecked(false)
		return
	}
	g.buttons[idx].SetChecked(true)
}

// handleCheckEvent is the first responder to state changes for the RadioButton
// instances associated with the group.
//
// If the event is an expected side effect of handling another event, this
// ignores the event. Otherwise, it calls g.react to update the state
// of g and any RadioButton instance that needs to change its state in
// response (which, if the event is "uncheck" and g is in "stay checked" mode,
// will be the instance that just told us it changed its state). It then calls
// all relevant user callbacks for RadioButton instances that actually changed
// state, and finally the user callback in hasFinishedChanging (if any state
// ultimately changed).
//
// State changes that are "resisted" due to stay-checked mode do not cause
// user callbacks to invoke. The user callback handlers are not invoked until
// g has finished changing its state, so they will see new values for Checked()
// and PreviouslyChecked().
func (g *RadioButtonGroup) handleCheckEvent(idx int, newState bool) {
	if g.ignoreEvent {
		return
	}

	unchecked, checked := g.react(idx, newState)
	if checked != nil {
		checked.changed(true)
	}
	if unchecked != nil {
		unchecked.changed(false)
	}

	if (checked != nil || unchecked != nil) && g.hasFinishedChanging != nil {
		g.hasFinishedChanging(unchecked, checked)
	}
}

// SetFinishedChangingFunc configures a callback to be invoked when the
// RadioButtonGroup changes which items it has checked. If the individual
// RadioButton instances involved have their own Changed handlers, those
// handlers are called first ("checked", then "unchecked"). The RadioButton
// instances that changed state are provided to the handler.
//
// If a RadioButton is unchecked directly (rather than because a new one was
// checked instead), `checked` will be nil. If no RadioButton in the group was
// previously checked (either because none had ever been checked before, or
// the prior change to the group unchecked the active item directly),
// unchecked will be nil.
func (g *RadioButtonGroup) SetFinishedChangingFunc(f func(unchecked, checked *RadioButton)) {
	g.hasFinishedChanging = f
}

// react updates checked-ness states of child RadioButton instances in response
// to a change callback from one of its children (which has already changed its
// state, although it may get changed back). It sets g.ignoreEvent for the
// duration of the call, which handleCheckEvent uses to filter out the events
// created as a side effect of changes that react itself imposes.
func (g *RadioButtonGroup) react(idx int, newState bool) (unchecked, checked *RadioButton) {
	if idx < 0 || idx >= len(g.buttons) {
		panic(fmt.Errorf("can't react to child %d (%t) from a RadioButtonGroup with %d children", idx, newState, len(g.buttons)))
	}
	if g.ignoreEvent {
		panic("react is not reentrant -- events should be discarded right now")
	}

	// Lock out events so changes made here don't produce more callbacks.
	g.ignoreEvent = true
	defer func() {
		g.ignoreEvent = false
	}()

	target := g.buttons[idx]

	if newState {
		// Checking a box.
		if g.checkedIdx == -1 {
			// No box to uncheck; just change our state.
			g.prevCheckedIdx = -1
			g.checkedIdx = idx
			return nil, target
		}

		// Uncheck the old box. Note that the event from doing this will be
		// discarded (due to g.ignoreEvent).
		former := g.buttons[g.checkedIdx]
		former.SetChecked(false)

		g.prevCheckedIdx = g.checkedIdx
		g.checkedIdx = idx
		return former, target
	}

	// unchecking a box. do we allow that?
	if g.stayChecked {
		// Resist the change. We'll get a callback for the re-checking
		// event, which we discard (due to ignoreEvent).
		target.SetChecked(true)

		// Do not invoke user handlers. Nothing changed.
		return nil, nil
	}

	// No other boxes need to be updated; just change our state.
	g.prevCheckedIdx = g.checkedIdx
	g.checkedIdx = -1
	return target, nil
}
