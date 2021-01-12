/* utils.h

   Free software by Richard W.E. Furse. Do with as you will. No
   warranty. */

#ifndef LADSPA_SDK_LOAD_PLUGIN_LIB
#define LADSPA_SDK_LOAD_PLUGIN_LIB

/*****************************************************************************/

#include "ladspa.h"

/*****************************************************************************/

/* Functions in load.c: */

/* This function call takes a plugin library filename, searches for
   the library along the LADSPA_PATH, loads it with dlopen() and
   returns a plugin handle for use with findPluginDescriptor() or
   unloadLADSPAPluginLibrary(). Errors are handled by writing a
   message to stderr and calling exit(1). It is alright (although
   inefficient) to call this more than once for the same file. */
void * loadLADSPAPluginLibrary(const char * pcPluginFilename);

/* This function unloads a LADSPA plugin library. */
void unloadLADSPAPluginLibrary(void * pvLADSPAPluginLibrary);

/* This function locates a LADSPA plugin within a plugin library
   loaded with loadLADSPAPluginLibrary(). Errors are handled by
   writing a message to stderr and calling exit(1). Note that the
   plugin library filename is only included to help provide
   informative error messages. */
const LADSPA_Descriptor *
findLADSPAPluginDescriptor(void * pvLADSPAPluginLibrary,
			   const char * pcPluginLibraryFilename,
			   const char * pcPluginLabel);

/*****************************************************************************/

/* Functions in search.c: */

/* Callback function for use with LADSPAPluginSearch(). The callback
   function passes the filename (full path), a plugin handle (dlopen()
   style) and a LADSPA_DescriptorFunction (from which
   LADSPA_Descriptors can be acquired). */
typedef void LADSPAPluginSearchCallbackFunction
(const char * pcFullFilename, 
 void * pvPluginHandle,
 LADSPA_Descriptor_Function fDescriptorFunction);

/* Search through the $(LADSPA_PATH) (or a default path) for any
   LADSPA plugin libraries. Each plugin library is tested using
   dlopen() and dlsym(,"ladspa_descriptor"). After loading each
   library, the callback function is called to process it. This
   function leaves items passed to the callback function open. */
void LADSPAPluginSearch(LADSPAPluginSearchCallbackFunction fCallbackFunction);

/*****************************************************************************/

/* Function in default.c: */

/* Find the default value for a port. Return 0 if a default is found
   and -1 if not. */
int getLADSPADefault(const LADSPA_PortRangeHint * psPortRangeHint,
		     const unsigned long          lSampleRate,
		     LADSPA_Data                * pfResult);


/*****************************************************************************/

/* During C pre-processing, take a string (passed in from the
   Makefile) and put quote marks around it. */
#define RAW_STRINGIFY(x) #x
#define EXPAND_AND_STRINGIFY(x) RAW_STRINGIFY(x)

/*****************************************************************************/

#ifndef __cplusplus
/* In C, special incantations are needed to trigger initialisation and
   cleanup routines when a dynamic plugin library is loaded or
   unloaded (e.g. with dlopen() or dlclose()). _init() and _fini() are
   classic exported symbols to achieve this, but these days GNU C
   likes to do things a different way. Ideally we would check the GNU
   version as older ones will probably expect the classic behaviour,
   but for now... */
# if __GNUC__
/* Modern GNU C incantations: */
#  define ON_LOAD_ROUTINE   static void __attribute__ ((constructor)) init()
#  define ON_UNLOAD_ROUTINE static void __attribute__ ((destructor))  fini()
# else
/* Classic incantations: */
#  define ON_LOAD_ROUTINE   void _init()
#  define ON_UNLOAD_ROUTINE void _fini()
# endif
#else
/* In C++, we use the constructor/destructor of a static object to
   manage initialisation and cleanup, so we don't need these
   routines. */
#endif

/*****************************************************************************/

#endif

/* EOF */
