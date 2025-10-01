package fit

import (
	"errors"

	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_trans "github.com/go-playground/validator/v10/translations/en"
	zh_trans "github.com/go-playground/validator/v10/translations/zh"
)

var validate *validator.Validate
var trans ut.Translator

func NewValidator(locale ...string) {
	zhTrans := zh.New()                      // Chinese converter
	enTrans := en.New()                      // English converter
	uni := ut.New(zhTrans, zhTrans, enTrans) // Universal converter
	defLocale := "zh"

	if len(locale) > 0 {
		defLocale = locale[0]
	}

	validate = validator.New()

	// Get the converter of the corresponding language
	trans, _ = uni.GetTranslator(defLocale)

	switch defLocale {
	case "zh":
		_ = zh_trans.RegisterDefaultTranslations(validate, trans)
	case "en":
		_ = en_trans.RegisterDefaultTranslations(validate, trans)
	default:
		_ = zh_trans.RegisterDefaultTranslations(validate, trans)
	}
}

func GetValidate() *validator.Validate {
	return validate
}

func Validate(data interface{}) error {
	err := validate.Struct(data)
	if err != nil {
		errs := err.(validator.ValidationErrors)
		for _, e := range errs {
			return errors.New(e.Translate(trans))
		}
	}
	return nil
}
