// Package all imports all kumokagi backend providers, triggering their init()
// registration with the factory. Import it with a blank identifier:
//
//	import _ "github.com/stlimtat/kumokagi/pkg/factory/all"
package all

import (
	_ "github.com/stlimtat/kumokagi/pkg/providers/aws"
	_ "github.com/stlimtat/kumokagi/pkg/providers/azure"
	_ "github.com/stlimtat/kumokagi/pkg/providers/gcp"
	_ "github.com/stlimtat/kumokagi/pkg/providers/onepassword"
	_ "github.com/stlimtat/kumokagi/pkg/providers/vault"
)
