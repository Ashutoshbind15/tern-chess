package common

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
)

func InitTextArea() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message here"
	ta.Focus()
	ta.Prompt = ">"
	ta.SetWidth(30)
	ta.SetHeight(3)
	ta.KeyMap.InsertNewline.SetEnabled(false)
	return ta
}

func InitTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Guest"
	ti.Focus()
	ti.Prompt = ">"
	return ti
}

func InitSpinner() spinner.Model {
	return spinner.New(spinner.WithSpinner(spinner.MiniDot))
}
