/*
  (c) Copyright 2021 github.com/lawl GPL3+
  Free software by Richard W.E. Furse. Do with as you will. No
  warranty.
*/

#include <math.h>
#include <stdlib.h>
#include <string.h>

#include "ladspa.h"
#include "utils.h"

#include "../c-ringbuf/ringbuf.h"
#include "../rnnoise/include/rnnoise.h"

#define SF_INPUT 0
#define SF_OUTPUT 1
#define SF_VAD 2

#define FRAMESIZE_NSAMPLES 480
#define FRAMESIZE_BYTES (480 * sizeof(float))

#define VAD_GRACE_PERIOD 20

typedef struct {

  DenoiseState *st;
  ringbuf_t in_buf;
  ringbuf_t out_buf;
  int32_t remaining_grace_period;
  int init;

  LADSPA_Data *m_pfVAD;
  LADSPA_Data *m_pfInput;
  LADSPA_Data *m_pfOutput;

} rnnoiseFilter;

static LADSPA_Handle
instantiateSimpleFilter(const LADSPA_Descriptor *Descriptor,
                        unsigned long SampleRate) {

  rnnoiseFilter *psFilter;

  psFilter = (rnnoiseFilter *)malloc(sizeof(rnnoiseFilter));

  if (psFilter) {
    psFilter->in_buf = ringbuf_new(FRAMESIZE_BYTES * 100);
    psFilter->out_buf = ringbuf_new(FRAMESIZE_BYTES * 100);
    psFilter->init = 0;
    psFilter->remaining_grace_period = VAD_GRACE_PERIOD;
    psFilter->st = rnnoise_create(NULL);
  }

  return psFilter;
}

static void activateSimpleFilter(LADSPA_Handle Instance) {
  
}

static void connectPortToSimpleFilter(LADSPA_Handle Instance,
                                      unsigned long Port,
                                      LADSPA_Data *DataLocation) {

  rnnoiseFilter *psFilter;

  psFilter = (rnnoiseFilter *)Instance;

  switch (Port) {
  case SF_VAD:
    psFilter->m_pfVAD = DataLocation;
    break;
  case SF_INPUT:
    psFilter->m_pfInput = DataLocation;
    break;
  case SF_OUTPUT:
    psFilter->m_pfOutput = DataLocation;
    break;
  }
}

static void runFilter(LADSPA_Handle Instance, unsigned long n_samples) {

  rnnoiseFilter *psFilter;

  psFilter = (rnnoiseFilter *)Instance;

  ringbuf_t in_buf = psFilter->in_buf;
  ringbuf_t out_buf = psFilter->out_buf;

  float *in, *out, vad_thresh;

  in = psFilter->m_pfInput;
  out = psFilter->m_pfOutput;

  vad_thresh = *psFilter->m_pfVAD / 100;

  for (int i = 0; i < n_samples; i++) {
    in[i] = in[i] * 32767;
  }

  ringbuf_memcpy_into(in_buf, in, n_samples * sizeof(float));

  const size_t n_frames = ringbuf_bytes_used(in_buf) / FRAMESIZE_BYTES;
  float tmpin[n_frames * FRAMESIZE_NSAMPLES];
  ringbuf_memcpy_from(tmpin, in_buf, FRAMESIZE_BYTES * n_frames);

  for (int i = 0; i < n_frames; i++) {
    float tmp[FRAMESIZE_NSAMPLES];
    float vad_prob = rnnoise_process_frame(psFilter->st, tmp,
                                           tmpin + (i * FRAMESIZE_NSAMPLES));
    if (vad_prob > vad_thresh) {
      psFilter->remaining_grace_period = VAD_GRACE_PERIOD;
    }

    if (psFilter->remaining_grace_period >= 0) {
      psFilter->remaining_grace_period--;
    } else {
      for (int i = 0; i < FRAMESIZE_NSAMPLES; i++) {
        tmp[i] = 0.f;
      }
    }
    ringbuf_memcpy_into(out_buf, tmp, FRAMESIZE_BYTES);
  }

  int frames_avail = ringbuf_bytes_used(out_buf) / FRAMESIZE_BYTES;
  int samples_avail = frames_avail * FRAMESIZE_NSAMPLES;

  if (samples_avail < n_samples) {
    int skip = n_samples - samples_avail;
    for (int i = 0; i < skip; i++) {
      out[i] = 0.f;
    }
    ringbuf_memcpy_from(out + skip, out_buf, samples_avail * sizeof(float));
  } else {
    ringbuf_memcpy_from(out, out_buf, n_samples * sizeof(float));
  }

  for (int i = 0; i < n_samples; i++) {
    out[i] = out[i] / 32767;
  }
}

static void cleanupFilter(LADSPA_Handle Instance) {
  rnnoiseFilter *psFilter = (rnnoiseFilter *)Instance;
  rnnoise_destroy(psFilter->st);
  ringbuf_free(&(psFilter->in_buf));
  ringbuf_free(&(psFilter->out_buf));
  free(Instance);
}

static LADSPA_Descriptor *g_psDescriptor = NULL;

ON_LOAD_ROUTINE {

  char **pcPortNames;
  LADSPA_PortDescriptor *piPortDescriptors;
  LADSPA_PortRangeHint *psPortRangeHints;

  g_psDescriptor = (LADSPA_Descriptor *)malloc(sizeof(LADSPA_Descriptor));

  if (g_psDescriptor != NULL) {

    g_psDescriptor->UniqueID = 16682994;
    g_psDescriptor->Label = strdup("nt-filter");
    g_psDescriptor->Properties = LADSPA_PROPERTY_HARD_RT_CAPABLE;
    g_psDescriptor->Name = strdup("nt-filter rnnoise ladspa module");
    g_psDescriptor->Maker = strdup("nt-org");
    g_psDescriptor->Copyright = strdup("GPL3+");
    g_psDescriptor->PortCount = 3;
    piPortDescriptors =
        (LADSPA_PortDescriptor *)calloc(3, sizeof(LADSPA_PortDescriptor));
    g_psDescriptor->PortDescriptors =
        (const LADSPA_PortDescriptor *)piPortDescriptors;
    piPortDescriptors[SF_VAD] = LADSPA_PORT_INPUT | LADSPA_PORT_CONTROL;
    piPortDescriptors[SF_INPUT] = LADSPA_PORT_INPUT | LADSPA_PORT_AUDIO;
    piPortDescriptors[SF_OUTPUT] = LADSPA_PORT_OUTPUT | LADSPA_PORT_AUDIO;
    pcPortNames = (char **)calloc(3, sizeof(char *));
    g_psDescriptor->PortNames = (const char **)pcPortNames;
    pcPortNames[SF_VAD] = strdup("VAD %%");
    pcPortNames[SF_INPUT] = strdup("Input");
    pcPortNames[SF_OUTPUT] = strdup("Output");
    psPortRangeHints =
        ((LADSPA_PortRangeHint *)calloc(3, sizeof(LADSPA_PortRangeHint)));
    g_psDescriptor->PortRangeHints =
        (const LADSPA_PortRangeHint *)psPortRangeHints;
    psPortRangeHints[SF_VAD].HintDescriptor =
        (LADSPA_HINT_BOUNDED_BELOW | LADSPA_HINT_BOUNDED_ABOVE);
    psPortRangeHints[SF_VAD].LowerBound = 0;
    psPortRangeHints[SF_VAD].UpperBound = 95;
    psPortRangeHints[SF_INPUT].HintDescriptor = 0;
    psPortRangeHints[SF_OUTPUT].HintDescriptor = 0;
    g_psDescriptor->instantiate = instantiateSimpleFilter;
    g_psDescriptor->connect_port = connectPortToSimpleFilter;
    g_psDescriptor->activate = activateSimpleFilter;
    g_psDescriptor->run = runFilter;
    g_psDescriptor->run_adding = NULL;
    g_psDescriptor->set_run_adding_gain = NULL;
    g_psDescriptor->deactivate = NULL;
    g_psDescriptor->cleanup = cleanupFilter;
  }
}

static void deleteDescriptor(LADSPA_Descriptor *psDescriptor) {
  unsigned long lIndex;
  if (psDescriptor) {
    free((char *)psDescriptor->Label);
    free((char *)psDescriptor->Name);
    free((char *)psDescriptor->Maker);
    free((char *)psDescriptor->Copyright);
    free((LADSPA_PortDescriptor *)psDescriptor->PortDescriptors);
    for (lIndex = 0; lIndex < psDescriptor->PortCount; lIndex++)
      free((char *)(psDescriptor->PortNames[lIndex]));
    free((char **)psDescriptor->PortNames);
    free((LADSPA_PortRangeHint *)psDescriptor->PortRangeHints);
    free(psDescriptor);
  }
}

ON_UNLOAD_ROUTINE { deleteDescriptor(g_psDescriptor); }

const LADSPA_Descriptor *ladspa_descriptor(unsigned long Index) {
  /* Return the requested descriptor or null if the index is out of
     range. */
  switch (Index) {
  case 0:
    return g_psDescriptor;
  default:
    return NULL;
  }
}
