package app

type Prompter interface {
	Password(prompt string) (string, error)
	Confirm(prompt string) (bool, error)
	Input(prompt, def string) (string, error)
	Select(prompt string, options []string) (int, error)
	Show(title, text string) error
}
