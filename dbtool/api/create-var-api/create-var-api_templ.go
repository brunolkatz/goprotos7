// Code generated by templ - DO NOT EDIT.

// templ: version: v0.3.865
package create_var_api

//lint:file-ignore SA4006 This context is only used if a nested component is present.

import "github.com/a-h/templ"
import templruntime "github.com/a-h/templ/runtime"

import (
	"fmt"
	"github.com/brunolkatz/goprotos7"
)

func CreateVarPageTempl() templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var1 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var1 == nil {
			templ_7745c5c3_Var1 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 1, "<div class=\"container-fluid p-5\"><div class=\"row mb-2\"><div class=\"col-12\"><div class=\"card\"><div class=\"card-header\">Create Variable</div><div class=\"card-body\"><form id=\"create-var-form\" hx-post=\"/vars/create-var\" hx-target=\"this\" hx-swap=\"outerHTML\"><div class=\"mb-3\"><label for=\"db-number\" class=\"form-label\">DB Number</label> <input type=\"number\" min=\"0\" class=\"form-control\" id=\"db-number\" name=\"db-number\" required></div><div class=\"mb-3\"><label for=\"name\" class=\"form-label\">Name</label> <input type=\"text\" class=\"form-control\" id=\"name\" name=\"name\" required></div><div class=\"mb-3\"><label for=\"description\" class=\"form-label\">Description</label> <input type=\"text\" class=\"form-control\" id=\"description\" name=\"description\" required></div><div class=\"mb-3\"><label for=\"data-type\" class=\"form-label\">Variable Type</label> <select class=\"form-select\" id=\"data-type\" name=\"data_type\" required hx-get=\"/vars/var-def\" hx-target=\"#var-type-def\"><option value=\"\" disabled selected>Select type</option> ")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		for _, t := range goprotos7.OrderedDataTypes {
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 2, "<option value=\"")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var2 string
			templ_7745c5c3_Var2, templ_7745c5c3_Err = templ.JoinStringErrs(fmt.Sprintf("%d", t))
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `api/create-var-api/create-var-api.templ`, Line: 47, Col: 76}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var2))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 3, "\">")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			var templ_7745c5c3_Var3 string
			templ_7745c5c3_Var3, templ_7745c5c3_Err = templ.JoinStringErrs(t.String())
			if templ_7745c5c3_Err != nil {
				return templ.Error{Err: templ_7745c5c3_Err, FileName: `api/create-var-api/create-var-api.templ`, Line: 47, Col: 91}
			}
			_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var3))
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
			templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 4, "</option>")
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 5, "</select></div><div class=\"row mb-2\"><div class=\"col-md-12\"><div id=\"var-type-def\"></div></div></div><button type=\"submit\" class=\"btn btn-primary\">Create Variable</button></form></div></div></div></div></div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

func CreateVarDefTempl(varType goprotos7.DataType) templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var4 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var4 == nil {
			templ_7745c5c3_Var4 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 6, "<div class=\"card\"><div class=\"card-header\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		var templ_7745c5c3_Var5 string
		templ_7745c5c3_Var5, templ_7745c5c3_Err = templ.JoinStringErrs(varType)
		if templ_7745c5c3_Err != nil {
			return templ.Error{Err: templ_7745c5c3_Err, FileName: `api/create-var-api/create-var-api.templ`, Line: 70, Col: 21}
		}
		_, templ_7745c5c3_Err = templ_7745c5c3_Buffer.WriteString(templ.EscapeString(templ_7745c5c3_Var5))
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 7, ": Type Definition</div><div class=\"card-body\">")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		switch varType {
		case goprotos7.BOOL:
			templ_7745c5c3_Err = DefineBoolVariable().Render(ctx, templ_7745c5c3_Buffer)
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		case goprotos7.SINT, goprotos7.USINT, goprotos7.INT, goprotos7.UINT, goprotos7.DINT, goprotos7.UDINT, goprotos7.LINT, goprotos7.ULINT:
			templ_7745c5c3_Err = DefineIntVariable().Render(ctx, templ_7745c5c3_Buffer)
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		case goprotos7.STRING:
			templ_7745c5c3_Err = DefineStrVariable().Render(ctx, templ_7745c5c3_Buffer)
			if templ_7745c5c3_Err != nil {
				return templ_7745c5c3_Err
			}
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 8, "</div></div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

func DefineStrVariable() templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var6 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var6 == nil {
			templ_7745c5c3_Var6 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 9, "<div class=\"input-group mb-3\"><label for=\"str-length\" class=\"form-label\">String Length</label> <input type=\"number\" id=\"str-length\" name=\"str-length\" class=\"form-control\" placeholder=\"\" aria-label=\"Default String Value\" required></div><div class=\"input-group mb-3\"><label for=\"str-default-value\" class=\"form-label\">Default Value</label> <input type=\"text\" id=\"str-default-value\" name=\"str-default-value\" class=\"form-control\" placeholder=\"\" aria-label=\"Default String Value\" required></div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

// DefineBoolVariable - Create the boolean fields for integer types
func DefineBoolVariable() templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var7 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var7 == nil {
			templ_7745c5c3_Var7 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 10, "<div id=\"bool-fields\"><!-- Default value --><div class=\"row mb-2\"><div class=\"col-12 col-md-12 col-lg-12 mb-3\"><label for=\"desc-bool-field[]\" class=\"form-label\">Bit description:</label> <input type=\"text\" id=\"desc-bool-field[]\" name=\"desc-bool-field[]\" placeholder=\"Bit description\" class=\"form-control\"></div><div class=\"col-md-6 mb-3\"><label for=\"bit-bool-field[]\" class=\"form-label\">Bit Offset</label> <input type=\"number\" id=\"bit-bool-field[]\" name=\"bit-bool-field[]\" placeholder=\"Description\" class=\"form-control\"></div><div class=\"col-md-6 mb-3\"><label for=\"bool-value[]\" class=\"form-label\">Default Value</label> <select id=\"bool-value[]\" name=\"bool-value[]\" class=\"form-select\" required><option value=\"\" disabled selected>Select Value</option> <option value=\"true\">True</option> <option value=\"false\">False</option></select></div><div class=\"col-12 col-lg-12 col-md-12\"><button class=\"btn btn-outline-secondary\" type=\"button\" onclick=\"addBoolField(&#39;bool-fields&#39;)\">Add New</button>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = AddBoolField().Render(ctx, templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 11, "</div></div></div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

func AddBoolField() templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var8 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var8 == nil {
			templ_7745c5c3_Var8 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 12, "<script>\n  // Create a new input group for the integer field\n  let lastBoolFieldId = 1\n  function addBoolField(divId) {\n    const container = document.getElementById(divId);\n    const fieldID = \"bool-field-\" + (lastBoolFieldId + 1);\n    const newDivID = `div-${fieldID}`\n\n    // Create the input group element\n    const newField = document.createElement(\"div\");\n    newField.id = newDivID;\n    newField.className = \"row mb-2\";\n\n    newField.innerHTML = `\n            <div class=\"col-12 col-md-12 col-lg-12 mb-3\">\n                <label for=\"desc-bool-field[]\" class=\"form-label\">Bit description:</label>\n                <input type=\"text\" id=\"desc-bool-field[]\" name=\"desc-bool-field[]\" placeholder=\"Bit description\" class=\"form-control\">\n            </div>\n            <div class=\"col-6 col-md-6 col-lg-6 mb-3\">\n                <label for=\"bit-bool-field[]\" class=\"form-label\">Bit Offset</label>\n                <input type=\"number\" id=\"bit-bool-field[]\" name=\"bit-bool-field[]\" placeholder=\"Bit offset\" class=\"form-control\">\n            </div>\n            <div class=\"col-6 col-md-6 col-lg-6 mb-3\">\n                <label for=\"bool-value[]\" class=\"form-label\">Bit Offset</label>\n                <select id=\"bool-value[]\" name=\"bool-value[]\" class=\"form-select\" required>\n                    <option value=\"\" disabled selected>Select default value...</option>\n                    <option value=\"true\">True</option>\n                    <option value=\"false\">False</option>\n                </select>\n            </div>\n            <div class=\"col-12 col-md-12 col-lg-12\">\n                <button class=\"btn btn-outline-secondary\" type=\"button\" onclick=\"addBoolField('bool-fields')\">Add New</button>\n                <button class=\"btn btn-outline-danger\" type=\"button\" onclick=\"removeBoolField('${newDivID}')\">Remove</button>\n            </div>\n        `;\n\n    // Append the new field to the specified div\n    container.appendChild(newField);\n    lastBoolFieldId += 1;\n  }\n  function removeBoolField(divId) {\n    const field = document.getElementById(divId);\n    if (field) {\n      field.remove();\n    }\n  }\n\n</script>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

// DefineIntVariable - Create the number fields for integer types
func DefineIntVariable() templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var9 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var9 == nil {
			templ_7745c5c3_Var9 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 13, "<div id=\"int-fields\"><!-- Default value --><div class=\"input-group mb-3\"><input type=\"number\" id=\"default-int-value\" name=\"default-int-value\" class=\"form-control\" placeholder=\"\" aria-label=\"Default Int Value\" required></div><div class=\"input-group mb-3\"><input type=\"text\" id=\"desc-int-field[]\" name=\"desc-int-field[]\" placeholder=\"Description\" aria-label=\"Variable value Description\" class=\"form-control\"> <input type=\"text\" id=\"int-field[]\" name=\"int-field[]\" class=\"form-control\" placeholder=\"\" aria-label=\"Variable int value\" aria-describedby=\"button-addon1\"> <button class=\"btn btn-outline-secondary\" type=\"button\" onclick=\"addIntField(&#39;int-fields&#39;)\">Add New</button>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = AddIntField().Render(ctx, templ_7745c5c3_Buffer)
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 14, "</div></div>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

func AddIntField() templ.Component {
	return templruntime.GeneratedTemplate(func(templ_7745c5c3_Input templruntime.GeneratedComponentInput) (templ_7745c5c3_Err error) {
		templ_7745c5c3_W, ctx := templ_7745c5c3_Input.Writer, templ_7745c5c3_Input.Context
		if templ_7745c5c3_CtxErr := ctx.Err(); templ_7745c5c3_CtxErr != nil {
			return templ_7745c5c3_CtxErr
		}
		templ_7745c5c3_Buffer, templ_7745c5c3_IsBuffer := templruntime.GetBuffer(templ_7745c5c3_W)
		if !templ_7745c5c3_IsBuffer {
			defer func() {
				templ_7745c5c3_BufErr := templruntime.ReleaseBuffer(templ_7745c5c3_Buffer)
				if templ_7745c5c3_Err == nil {
					templ_7745c5c3_Err = templ_7745c5c3_BufErr
				}
			}()
		}
		ctx = templ.InitializeContext(ctx)
		templ_7745c5c3_Var10 := templ.GetChildren(ctx)
		if templ_7745c5c3_Var10 == nil {
			templ_7745c5c3_Var10 = templ.NopComponent
		}
		ctx = templ.ClearChildren(ctx)
		templ_7745c5c3_Err = templruntime.WriteString(templ_7745c5c3_Buffer, 15, "<script>\n    // Create a new input group for the integer field\n    let lastIntFieldId = 1\n    function addIntField(divId) {\n        const container = document.getElementById(divId);\n        const fieldID = \"int-field-\" + (lastIntFieldId + 1);\n        const newDivID = `div-${fieldID}`\n\n        // Create the input group element\n        const newField = document.createElement(\"div\");\n        newField.id = newDivID;\n        newField.className = \"input-group mb-3\";\n\n        newField.innerHTML = `\n            <input type=\"text\" id=\"desc-int-field[]\" name=\"desc-int-field[]\" placeholder=\"Description\" aria-label=\"Variable value Description\" class=\"form-control\" required>\n            <input type=\"number\" id=\"int-field[]\" name=\"int-field[]\" aria-placeholder=\"Value Number\" class=\"form-control\" placeholder=\"\" aria-label=\"Variable Int Value\" aria-describedby=\"button-addon1\" required>\n            <button class=\"btn btn-outline-secondary\" type=\"button\" onclick=\"addIntField('int-fields')\">Add New</button>\n            <button class=\"btn btn-outline-secondary\" type=\"button\" onclick=\"removeIntField('${newDivID}')\">Remove</button>\n        `;\n\n        // Append the new field to the specified div\n        container.appendChild(newField);\n        lastIntFieldId += 1;\n    }\n    function removeIntField(divId) {\n        const field = document.getElementById(divId);\n        if (field) {\n            field.remove();\n        }\n    }\n\n    </script>")
		if templ_7745c5c3_Err != nil {
			return templ_7745c5c3_Err
		}
		return nil
	})
}

var _ = templruntime.GeneratedTemplate
