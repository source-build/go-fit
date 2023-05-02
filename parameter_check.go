package fit

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_trans "github.com/go-playground/validator/v10/translations/en"
	zh_trans "github.com/go-playground/validator/v10/translations/zh"
)

var validate *validator.Validate
var trans ut.Translator

func NewValidator() {
	zhTrans := zh.New()                      // Chinese converter
	enTrans := en.New()                      // English converter
	uni := ut.New(zhTrans, zhTrans, enTrans) // Universal converter
	curLocales := "zh"                       // Set current language type
	trans, _ = uni.GetTranslator(curLocales) // Get the converter of the corresponding language
	validate = validator.New()
	switch curLocales {
	case "zh":
		_ = zh_trans.RegisterDefaultTranslations(validate, trans)
	case "en":
		_ = en_trans.RegisterDefaultTranslations(validate, trans)
	}
}

func CheckParam(c *gin.Context, data interface{}) error {
	err := c.ShouldBindJSON(&data)
	if err != nil {
		return err
	}

	err = validate.Struct(data)
	if err != nil {
		errs := err.(validator.ValidationErrors)
		for _, e := range errs {
			return errors.New(e.Translate(trans))
		}

	}
	return nil
}

func CheckParamXML(c *gin.Context, data interface{}) error {
	err := c.ShouldBindXML(&data)
	if err != nil {
		return err
	}

	err = validate.Struct(data)
	if err != nil {
		errs := err.(validator.ValidationErrors)
		for _, e := range errs {
			return errors.New(e.Translate(trans))
		}

	}
	return nil
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
