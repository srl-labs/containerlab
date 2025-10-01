package network

import (
	"fmt"
	"regexp"

	"github.com/scrapli/scrapligo/channel"
	"github.com/scrapli/scrapligo/util"
)

const (
	noAction         = "noAction"
	escalateAction   = "escalateAction"
	deescalateAction = "deescalateAction"

	unknownPriv = "UNKNOWN"
)

func (d *Driver) buildPrivChangeMap(current, target string, steps *[]string) []string {
	var workingSteps []string

	if steps != nil {
		workingSteps = *steps
	}

	workingSteps = append(workingSteps, current)

	if current == target {
		return workingSteps
	}

	for priv := range d.privGraph[current] {
		if !util.StringSliceContains(workingSteps, priv) {
			newWorkingSteps := d.buildPrivChangeMap(priv, target, &workingSteps)
			if len(newWorkingSteps) > 0 {
				return newWorkingSteps
			}
		}
	}

	return nil
}

func (d *Driver) determineCurrentPriv(currentPrompt string) ([]string, error) {
	var possiblePrivs []string

	for _, priv := range d.PrivilegeLevels {
		if util.StringContainsAny(currentPrompt, priv.NotContains) {
			continue
		}

		if priv.patternRe.MatchString(currentPrompt) {
			possiblePrivs = append(possiblePrivs, priv.Name)
		}
	}

	if len(possiblePrivs) == 0 {
		return nil, fmt.Errorf(
			"%w: could not determine privilege level from prompt '%s'",
			util.ErrPrivilegeError, currentPrompt,
		)
	}

	return possiblePrivs, nil
}

func (d *Driver) processAcquirePriv(
	target, currentPrompt string,
) (action, nextPriv string, err error) {
	possiblePrivs, err := d.determineCurrentPriv(currentPrompt)
	if err != nil {
		return "", "", err
	}

	var current string

	switch {
	case util.StringSliceContains(possiblePrivs, d.CurrentPriv):
		current = d.CurrentPriv
	case util.StringSliceContains(possiblePrivs, target):
		current = d.PrivilegeLevels[target].Name
	default:
		current = possiblePrivs[0]
	}

	if current == target {
		d.CurrentPriv = current

		return noAction, current, nil
	}

	mapTo := d.buildPrivChangeMap(current, target, nil)

	// at this point we basically dont *know* the privilege leve we are at (or we wont/cant after
	// we do an escalation or deescalation, so we reset to the dummy priv level
	d.CurrentPriv = unknownPriv

	if d.PrivilegeLevels[mapTo[1]].PreviousPriv != current {
		return deescalateAction, current, nil
	}

	return escalateAction, d.PrivilegeLevels[mapTo[1]].Name, nil
}

func (d *Driver) escalate(target string) error {
	var err error

	p := d.PrivilegeLevels[target]

	if !p.EscalateAuth || d.AuthSecondary == "" {
		if d.AuthSecondary == "" {
			d.Logger.Info(
				"no auth secondary set, but escalate target may require auth," +
					" trying with no password",
			)
		}

		_, err = d.Driver.Channel.SendInput(p.Escalate)
	} else {
		events := []*channel.SendInteractiveEvent{
			{
				ChannelInput:    p.Escalate,
				ChannelResponse: p.EscalatePrompt,
				HideInput:       false,
			},
			{
				ChannelInput:    d.AuthSecondary,
				ChannelResponse: p.Pattern,
				HideInput:       true,
			},
		}

		_, err = d.Driver.Channel.SendInteractive(
			events,
			// can't import opoptions as we'll have recursive imports, so we'll just do this...
			// options probably could be handled more nicely, but it seems nice to have them in a
			// single place so users don't have to think about where to import which option from,
			// if this is the price we pay for that then it seems ok.
			func(o interface{}) error {
				a, ok := o.(*channel.OperationOptions)

				if ok {
					a.CompletePatterns = []*regexp.Regexp{
						d.PrivilegeLevels[p.PreviousPriv].patternRe,
						p.patternRe,
					}

					return nil
				}

				return util.ErrIgnoredOption
			},
		)
	}

	return err
}

func (d *Driver) deescalate(target string) error {
	p := d.PrivilegeLevels[target]

	_, err := d.Driver.Channel.SendInput(p.Deescalate)

	return err
}

// AcquirePriv acquires the privilege level target. This method will handle any escalation or
// deescalation necessary to acquire the requested privilege level including any authentication
// that may be required.
func (d *Driver) AcquirePriv(target string) error {
	d.Logger.Infof("AcquirePriv requested, target privilege level '%s'", target)

	if _, ok := d.PrivilegeLevels[target]; !ok {
		return fmt.Errorf(
			"%w: requested target privilege level '%s' is not a valid privilege level",
			util.ErrPrivilegeError,
			target,
		)
	}

	var count int

	for {
		currentPrompt, err := d.Driver.GetPrompt()
		if err != nil {
			return err
		}

		action, next, err := d.processAcquirePriv(
			target,
			currentPrompt,
		)
		if err != nil {
			return err
		}

		switch action {
		case noAction:
			d.Logger.Debug("AcquirePriv determined no privilege action necessary")

			return nil
		case escalateAction:
			d.Logger.Debug("AcquirePriv determined privilege escalation necessary")

			err = d.escalate(next)
		case deescalateAction:
			d.Logger.Debug("AcquirePriv determined privilege de-escalation necessary")

			err = d.deescalate(next)
		}

		if err != nil {
			return err
		}

		count++

		if count > len(d.PrivilegeLevels)*2 {
			return fmt.Errorf(
				"%w: failed to acquire target privilege level '%s'",
				util.ErrPrivilegeError,
				target,
			)
		}
	}
}
