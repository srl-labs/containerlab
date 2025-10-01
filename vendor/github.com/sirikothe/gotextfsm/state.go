package gotextfsm

type TextFSMState struct {
	name  string
	rules []TextFSMRule
	fsm   *TextFSM
}
