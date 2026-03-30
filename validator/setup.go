package validator

func setup() {
	if err := RegisterValidationRules(presetValidationRules...); err != nil {
		panic(err)
	}
}
