// Simple wrapper to translate the API exposed methods and types to inthernal
// Go versions of the same types.

#include "_cgo_export.h"

int run(const char* args) {
 return doRun((char*)args);
}
