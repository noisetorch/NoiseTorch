// SPDX-License-Identifier: Unlicense OR MIT

package window

/*
#cgo CFLAGS: -Werror
#cgo LDFLAGS: -landroid

#include <android/native_window_jni.h>
#include <android/configuration.h>
#include <android/keycodes.h>
#include <android/input.h>
#include <stdlib.h>

__attribute__ ((visibility ("hidden"))) jint gio_jni_GetEnv(JavaVM *vm, JNIEnv **env, jint version);
__attribute__ ((visibility ("hidden"))) jint gio_jni_GetJavaVM(JNIEnv *env, JavaVM **jvm);
__attribute__ ((visibility ("hidden"))) jint gio_jni_AttachCurrentThread(JavaVM *vm, JNIEnv **p_env, void *thr_args);
__attribute__ ((visibility ("hidden"))) jint gio_jni_DetachCurrentThread(JavaVM *vm);

__attribute__ ((visibility ("hidden"))) jobject gio_jni_NewGlobalRef(JNIEnv *env, jobject obj);
__attribute__ ((visibility ("hidden"))) void gio_jni_DeleteGlobalRef(JNIEnv *env, jobject obj);
__attribute__ ((visibility ("hidden"))) jclass gio_jni_GetObjectClass(JNIEnv *env, jobject obj);
__attribute__ ((visibility ("hidden"))) jmethodID gio_jni_GetStaticMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig);
__attribute__ ((visibility ("hidden"))) jmethodID gio_jni_GetMethodID(JNIEnv *env, jclass clazz, const char *name, const char *sig);
__attribute__ ((visibility ("hidden"))) jfloat gio_jni_CallFloatMethod(JNIEnv *env, jobject obj, jmethodID methodID);
__attribute__ ((visibility ("hidden"))) jint gio_jni_CallIntMethod(JNIEnv *env, jobject obj, jmethodID methodID);
__attribute__ ((visibility ("hidden"))) void gio_jni_CallStaticVoidMethodA(JNIEnv *env, jclass cls, jmethodID methodID, const jvalue *args);
__attribute__ ((visibility ("hidden"))) void gio_jni_CallVoidMethodA(JNIEnv *env, jobject obj, jmethodID methodID, const jvalue *args);
__attribute__ ((visibility ("hidden"))) jbyte *gio_jni_GetByteArrayElements(JNIEnv *env, jbyteArray arr);
__attribute__ ((visibility ("hidden"))) void gio_jni_ReleaseByteArrayElements(JNIEnv *env, jbyteArray arr, jbyte *bytes);
__attribute__ ((visibility ("hidden"))) jsize gio_jni_GetArrayLength(JNIEnv *env, jbyteArray arr);
__attribute__ ((visibility ("hidden"))) jstring gio_jni_NewString(JNIEnv *env, const jchar *unicodeChars, jsize len);
__attribute__ ((visibility ("hidden"))) jsize gio_jni_GetStringLength(JNIEnv *env, jstring str);
__attribute__ ((visibility ("hidden"))) const jchar *gio_jni_GetStringChars(JNIEnv *env, jstring str);
__attribute__ ((visibility ("hidden"))) jthrowable gio_jni_ExceptionOccurred(JNIEnv *env);
__attribute__ ((visibility ("hidden"))) void gio_jni_ExceptionClear(JNIEnv *env);
__attribute__ ((visibility ("hidden"))) jobject gio_jni_CallObjectMethodA(JNIEnv *env, jobject obj, jmethodID method, jvalue *args);
__attribute__ ((visibility ("hidden"))) jobject gio_jni_CallStaticObjectMethodA(JNIEnv *env, jclass cls, jmethodID method, jvalue *args);
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"reflect"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/unit"
)

type window struct {
	callbacks Callbacks

	view C.jobject

	dpi       int
	fontScale float32
	insets    system.Insets

	stage   system.Stage
	started bool

	mu        sync.Mutex
	win       *C.ANativeWindow
	animating bool

	// Cached Java methods.
	mgetDensity        C.jmethodID
	mgetFontScale      C.jmethodID
	mshowTextInput     C.jmethodID
	mhideTextInput     C.jmethodID
	mpostFrameCallback C.jmethodID
}

type jvalue uint64 // The largest JNI type fits in 64 bits.

var dataDirChan = make(chan string, 1)

var android struct {
	// mu protects all fields of this structure. However, once a
	// non-nil jvm is returned from javaVM, all the other fields may
	// be accessed unlocked.
	mu  sync.Mutex
	jvm *C.JavaVM

	// appCtx is the global Android App context.
	appCtx C.jobject
	// gioCls is the class of the Gio class.
	gioCls C.jclass

	mwriteClipboard   C.jmethodID
	mreadClipboard    C.jmethodID
	mwakeupMainThread C.jmethodID
}

var views = make(map[C.jlong]*window)

var mainWindow = newWindowRendezvous()

var mainFuncs = make(chan func(env *C.JNIEnv), 1)

func getMethodID(env *C.JNIEnv, class C.jclass, method, sig string) C.jmethodID {
	m := C.CString(method)
	defer C.free(unsafe.Pointer(m))
	s := C.CString(sig)
	defer C.free(unsafe.Pointer(s))
	jm := C.gio_jni_GetMethodID(env, class, m, s)
	if err := exception(env); err != nil {
		panic(err)
	}
	return jm
}

func getStaticMethodID(env *C.JNIEnv, class C.jclass, method, sig string) C.jmethodID {
	m := C.CString(method)
	defer C.free(unsafe.Pointer(m))
	s := C.CString(sig)
	defer C.free(unsafe.Pointer(s))
	jm := C.gio_jni_GetStaticMethodID(env, class, m, s)
	if err := exception(env); err != nil {
		panic(err)
	}
	return jm
}

//export Java_org_gioui_Gio_runGoMain
func Java_org_gioui_Gio_runGoMain(env *C.JNIEnv, class C.jclass, jdataDir C.jbyteArray, context C.jobject) {
	initJVM(env, class, context)
	dirBytes := C.gio_jni_GetByteArrayElements(env, jdataDir)
	if dirBytes == nil {
		panic("runGoMain: GetByteArrayElements failed")
	}
	n := C.gio_jni_GetArrayLength(env, jdataDir)
	dataDir := C.GoStringN((*C.char)(unsafe.Pointer(dirBytes)), n)
	dataDirChan <- dataDir
	C.gio_jni_ReleaseByteArrayElements(env, jdataDir, dirBytes)

	runMain()
}

func initJVM(env *C.JNIEnv, gio C.jclass, ctx C.jobject) {
	android.mu.Lock()
	defer android.mu.Unlock()
	if res := C.gio_jni_GetJavaVM(env, &android.jvm); res != 0 {
		panic("gio: GetJavaVM failed")
	}
	android.appCtx = C.gio_jni_NewGlobalRef(env, ctx)
	android.gioCls = C.jclass(C.gio_jni_NewGlobalRef(env, C.jobject(gio)))
	android.mwriteClipboard = getStaticMethodID(env, gio, "writeClipboard", "(Landroid/content/Context;Ljava/lang/String;)V")
	android.mreadClipboard = getStaticMethodID(env, gio, "readClipboard", "(Landroid/content/Context;)Ljava/lang/String;")
	android.mwakeupMainThread = getStaticMethodID(env, gio, "wakeupMainThread", "()V")
}

func JavaVM() uintptr {
	jvm := javaVM()
	return uintptr(unsafe.Pointer(jvm))
}

func javaVM() *C.JavaVM {
	android.mu.Lock()
	defer android.mu.Unlock()
	return android.jvm
}

func AppContext() uintptr {
	android.mu.Lock()
	defer android.mu.Unlock()
	return uintptr(android.appCtx)
}

func GetDataDir() string {
	return <-dataDirChan
}

//export Java_org_gioui_GioView_onCreateView
func Java_org_gioui_GioView_onCreateView(env *C.JNIEnv, class C.jclass, view C.jobject) C.jlong {
	view = C.gio_jni_NewGlobalRef(env, view)
	w := &window{
		view:               view,
		mgetDensity:        getMethodID(env, class, "getDensity", "()I"),
		mgetFontScale:      getMethodID(env, class, "getFontScale", "()F"),
		mshowTextInput:     getMethodID(env, class, "showTextInput", "()V"),
		mhideTextInput:     getMethodID(env, class, "hideTextInput", "()V"),
		mpostFrameCallback: getMethodID(env, class, "postFrameCallback", "()V"),
	}
	wopts := <-mainWindow.out
	w.callbacks = wopts.window
	w.callbacks.SetDriver(w)
	handle := C.jlong(view)
	views[handle] = w
	w.loadConfig(env, class)
	w.setStage(system.StagePaused)
	return handle
}

//export Java_org_gioui_GioView_onDestroyView
func Java_org_gioui_GioView_onDestroyView(env *C.JNIEnv, class C.jclass, handle C.jlong) {
	w := views[handle]
	w.callbacks.SetDriver(nil)
	delete(views, handle)
	C.gio_jni_DeleteGlobalRef(env, w.view)
	w.view = 0
}

//export Java_org_gioui_GioView_onStopView
func Java_org_gioui_GioView_onStopView(env *C.JNIEnv, class C.jclass, handle C.jlong) {
	w := views[handle]
	w.started = false
	w.setStage(system.StagePaused)
}

//export Java_org_gioui_GioView_onStartView
func Java_org_gioui_GioView_onStartView(env *C.JNIEnv, class C.jclass, handle C.jlong) {
	w := views[handle]
	w.started = true
	if w.aNativeWindow() != nil {
		w.setVisible()
	}
}

//export Java_org_gioui_GioView_onSurfaceDestroyed
func Java_org_gioui_GioView_onSurfaceDestroyed(env *C.JNIEnv, class C.jclass, handle C.jlong) {
	w := views[handle]
	w.mu.Lock()
	w.win = nil
	w.mu.Unlock()
	w.setStage(system.StagePaused)
}

//export Java_org_gioui_GioView_onSurfaceChanged
func Java_org_gioui_GioView_onSurfaceChanged(env *C.JNIEnv, class C.jclass, handle C.jlong, surf C.jobject) {
	w := views[handle]
	w.mu.Lock()
	w.win = C.ANativeWindow_fromSurface(env, surf)
	w.mu.Unlock()
	if w.started {
		w.setVisible()
	}
}

//export Java_org_gioui_GioView_onLowMemory
func Java_org_gioui_GioView_onLowMemory() {
	runtime.GC()
	debug.FreeOSMemory()
}

//export Java_org_gioui_GioView_onConfigurationChanged
func Java_org_gioui_GioView_onConfigurationChanged(env *C.JNIEnv, class C.jclass, view C.jlong) {
	w := views[view]
	w.loadConfig(env, class)
	if w.stage >= system.StageRunning {
		w.draw(true)
	}
}

//export Java_org_gioui_GioView_onFrameCallback
func Java_org_gioui_GioView_onFrameCallback(env *C.JNIEnv, class C.jclass, view C.jlong, nanos C.jlong) {
	w, exist := views[view]
	if !exist {
		return
	}
	if w.stage < system.StageRunning {
		return
	}
	w.mu.Lock()
	anim := w.animating
	w.mu.Unlock()
	if anim {
		runInJVM(javaVM(), func(env *C.JNIEnv) {
			callVoidMethod(env, w.view, w.mpostFrameCallback)
		})
		w.draw(false)
	}
}

//export Java_org_gioui_GioView_onBack
func Java_org_gioui_GioView_onBack(env *C.JNIEnv, class C.jclass, view C.jlong) C.jboolean {
	w := views[view]
	ev := &system.CommandEvent{Type: system.CommandBack}
	w.callbacks.Event(ev)
	if ev.Cancel {
		return C.JNI_TRUE
	}
	return C.JNI_FALSE
}

//export Java_org_gioui_GioView_onFocusChange
func Java_org_gioui_GioView_onFocusChange(env *C.JNIEnv, class C.jclass, view C.jlong, focus C.jboolean) {
	w := views[view]
	w.callbacks.Event(key.FocusEvent{Focus: focus == C.JNI_TRUE})
}

//export Java_org_gioui_GioView_onWindowInsets
func Java_org_gioui_GioView_onWindowInsets(env *C.JNIEnv, class C.jclass, view C.jlong, top, right, bottom, left C.jint) {
	w := views[view]
	w.insets = system.Insets{
		Top:    unit.Px(float32(top)),
		Right:  unit.Px(float32(right)),
		Bottom: unit.Px(float32(bottom)),
		Left:   unit.Px(float32(left)),
	}
	if w.stage >= system.StageRunning {
		w.draw(true)
	}
}

func (w *window) setVisible() {
	win := w.aNativeWindow()
	width, height := C.ANativeWindow_getWidth(win), C.ANativeWindow_getHeight(win)
	if width == 0 || height == 0 {
		return
	}
	w.setStage(system.StageRunning)
	w.draw(true)
}

func (w *window) setStage(stage system.Stage) {
	if stage == w.stage {
		return
	}
	w.stage = stage
	w.callbacks.Event(system.StageEvent{stage})
}

func (w *window) nativeWindow(visID int) (*C.ANativeWindow, int, int) {
	win := w.aNativeWindow()
	var width, height int
	if win != nil {
		if C.ANativeWindow_setBuffersGeometry(win, 0, 0, C.int32_t(visID)) != 0 {
			panic(errors.New("ANativeWindow_setBuffersGeometry failed"))
		}
		w, h := C.ANativeWindow_getWidth(win), C.ANativeWindow_getHeight(win)
		width, height = int(w), int(h)
	}
	return win, width, height
}

func (w *window) aNativeWindow() *C.ANativeWindow {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.win
}

func (w *window) loadConfig(env *C.JNIEnv, class C.jclass) {
	dpi := int(C.gio_jni_CallIntMethod(env, w.view, w.mgetDensity))
	w.fontScale = float32(C.gio_jni_CallFloatMethod(env, w.view, w.mgetFontScale))
	switch dpi {
	case C.ACONFIGURATION_DENSITY_NONE,
		C.ACONFIGURATION_DENSITY_DEFAULT,
		C.ACONFIGURATION_DENSITY_ANY:
		// Assume standard density.
		w.dpi = C.ACONFIGURATION_DENSITY_MEDIUM
	default:
		w.dpi = int(dpi)
	}
}

func (w *window) SetAnimating(anim bool) {
	w.mu.Lock()
	w.animating = anim
	w.mu.Unlock()
	if anim {
		runOnMain(func(env *C.JNIEnv) {
			if w.view == 0 {
				// View was destroyed while switching to main thread.
				return
			}
			callVoidMethod(env, w.view, w.mpostFrameCallback)
		})
	}
}

func (w *window) draw(sync bool) {
	win := w.aNativeWindow()
	width, height := C.ANativeWindow_getWidth(win), C.ANativeWindow_getHeight(win)
	if width == 0 || height == 0 {
		return
	}
	const inchPrDp = 1.0 / 160
	ppdp := float32(w.dpi) * inchPrDp
	w.callbacks.Event(FrameEvent{
		FrameEvent: system.FrameEvent{
			Now: time.Now(),
			Size: image.Point{
				X: int(width),
				Y: int(height),
			},
			Insets: w.insets,
			Metric: unit.Metric{
				PxPerDp: ppdp,
				PxPerSp: w.fontScale * ppdp,
			},
		},
		Sync: sync,
	})
}

type keyMapper func(devId, keyCode C.int32_t) rune

func runInJVM(jvm *C.JavaVM, f func(env *C.JNIEnv)) {
	if jvm == nil {
		panic("nil JVM")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var env *C.JNIEnv
	if res := C.gio_jni_GetEnv(jvm, &env, C.JNI_VERSION_1_6); res != C.JNI_OK {
		if res != C.JNI_EDETACHED {
			panic(fmt.Errorf("JNI GetEnv failed with error %d", res))
		}
		if C.gio_jni_AttachCurrentThread(jvm, &env, nil) != C.JNI_OK {
			panic(errors.New("runInJVM: AttachCurrentThread failed"))
		}
		defer C.gio_jni_DetachCurrentThread(jvm)
	}

	f(env)
}

func convertKeyCode(code C.jint) (string, bool) {
	var n string
	switch code {
	case C.AKEYCODE_DPAD_UP:
		n = key.NameUpArrow
	case C.AKEYCODE_DPAD_DOWN:
		n = key.NameDownArrow
	case C.AKEYCODE_DPAD_LEFT:
		n = key.NameLeftArrow
	case C.AKEYCODE_DPAD_RIGHT:
		n = key.NameRightArrow
	case C.AKEYCODE_FORWARD_DEL:
		n = key.NameDeleteForward
	case C.AKEYCODE_DEL:
		n = key.NameDeleteBackward
	default:
		return "", false
	}
	return n, true
}

//export Java_org_gioui_GioView_onKeyEvent
func Java_org_gioui_GioView_onKeyEvent(env *C.JNIEnv, class C.jclass, handle C.jlong, keyCode, r C.jint, t C.jlong) {
	w := views[handle]
	if n, ok := convertKeyCode(keyCode); ok {
		w.callbacks.Event(key.Event{Name: n})
	}
	if r != 0 {
		w.callbacks.Event(key.EditEvent{Text: string(rune(r))})
	}
}

//export Java_org_gioui_GioView_onTouchEvent
func Java_org_gioui_GioView_onTouchEvent(env *C.JNIEnv, class C.jclass, handle C.jlong, action, pointerID, tool C.jint, x, y, scrollX, scrollY C.jfloat, jbtns C.jint, t C.jlong) {
	w := views[handle]
	var typ pointer.Type
	switch action {
	case C.AMOTION_EVENT_ACTION_DOWN, C.AMOTION_EVENT_ACTION_POINTER_DOWN:
		typ = pointer.Press
	case C.AMOTION_EVENT_ACTION_UP, C.AMOTION_EVENT_ACTION_POINTER_UP:
		typ = pointer.Release
	case C.AMOTION_EVENT_ACTION_CANCEL:
		typ = pointer.Cancel
	case C.AMOTION_EVENT_ACTION_MOVE:
		typ = pointer.Move
	case C.AMOTION_EVENT_ACTION_SCROLL:
		typ = pointer.Scroll
	default:
		return
	}
	var src pointer.Source
	var btns pointer.Buttons
	if jbtns&C.AMOTION_EVENT_BUTTON_PRIMARY != 0 {
		btns |= pointer.ButtonLeft
	}
	if jbtns&C.AMOTION_EVENT_BUTTON_SECONDARY != 0 {
		btns |= pointer.ButtonRight
	}
	if jbtns&C.AMOTION_EVENT_BUTTON_TERTIARY != 0 {
		btns |= pointer.ButtonMiddle
	}
	switch tool {
	case C.AMOTION_EVENT_TOOL_TYPE_FINGER:
		src = pointer.Touch
	case C.AMOTION_EVENT_TOOL_TYPE_MOUSE:
		src = pointer.Mouse
	case C.AMOTION_EVENT_TOOL_TYPE_UNKNOWN:
		// For example, triggered via 'adb shell input tap'.
		// Instead of discarding it, treat it as a touch event.
		src = pointer.Touch
	default:
		return
	}
	w.callbacks.Event(pointer.Event{
		Type:      typ,
		Source:    src,
		Buttons:   btns,
		PointerID: pointer.ID(pointerID),
		Time:      time.Duration(t) * time.Millisecond,
		Position:  f32.Point{X: float32(x), Y: float32(y)},
		Scroll:    f32.Pt(float32(scrollX), float32(scrollY)),
	})
}

func (w *window) ShowTextInput(show bool) {
	if w.view == 0 {
		return
	}
	runInJVM(javaVM(), func(env *C.JNIEnv) {
		if show {
			callVoidMethod(env, w.view, w.mshowTextInput)
		} else {
			callVoidMethod(env, w.view, w.mhideTextInput)
		}
	})
}

func javaString(env *C.JNIEnv, str string) C.jstring {
	if str == "" {
		return 0
	}
	utf16Chars := utf16.Encode([]rune(str))
	return C.gio_jni_NewString(env, (*C.jchar)(unsafe.Pointer(&utf16Chars[0])), C.int(len(utf16Chars)))
}

// Do invokes the function with a global JNI handle to the view. If
// the view is destroyed, Do returns false and does not invoke the
// function.
//
// NOTE: Do must be invoked on the Android main thread.
func (w *window) Do(f func(view uintptr)) bool {
	if w.view == 0 {
		return false
	}
	runInJVM(javaVM(), func(env *C.JNIEnv) {
		view := C.gio_jni_NewGlobalRef(env, w.view)
		defer C.gio_jni_DeleteGlobalRef(env, view)
		f(uintptr(view))
	})
	return true
}

func varArgs(args []jvalue) *C.jvalue {
	if len(args) == 0 {
		return nil
	}
	return (*C.jvalue)(unsafe.Pointer(&args[0]))
}

func callStaticVoidMethod(env *C.JNIEnv, cls C.jclass, method C.jmethodID, args ...jvalue) error {
	C.gio_jni_CallStaticVoidMethodA(env, cls, method, varArgs(args))
	return exception(env)
}

func callStaticObjectMethod(env *C.JNIEnv, cls C.jclass, method C.jmethodID, args ...jvalue) (C.jobject, error) {
	res := C.gio_jni_CallStaticObjectMethodA(env, cls, method, varArgs(args))
	return res, exception(env)
}

func callVoidMethod(env *C.JNIEnv, obj C.jobject, method C.jmethodID, args ...jvalue) error {
	C.gio_jni_CallVoidMethodA(env, obj, method, varArgs(args))
	return exception(env)
}

func callObjectMethod(env *C.JNIEnv, obj C.jobject, method C.jmethodID, args ...jvalue) (C.jobject, error) {
	res := C.gio_jni_CallObjectMethodA(env, obj, method, varArgs(args))
	return res, exception(env)
}

// exception returns an error corresponding to the pending
// exception, or nil if no exception is pending. The pending
// exception is cleared.
func exception(env *C.JNIEnv) error {
	thr := C.gio_jni_ExceptionOccurred(env)
	if thr == 0 {
		return nil
	}
	C.gio_jni_ExceptionClear(env)
	cls := getObjectClass(env, C.jobject(thr))
	toString := getMethodID(env, cls, "toString", "()Ljava/lang/String;")
	msg, err := callObjectMethod(env, C.jobject(thr), toString)
	if err != nil {
		return err
	}
	return errors.New(goString(env, C.jstring(msg)))
}

func getObjectClass(env *C.JNIEnv, obj C.jobject) C.jclass {
	if obj == 0 {
		panic("null object")
	}
	cls := C.gio_jni_GetObjectClass(env, C.jobject(obj))
	if err := exception(env); err != nil {
		// GetObjectClass should never fail.
		panic(err)
	}
	return cls
}

// goString converts the JVM jstring to a Go string.
func goString(env *C.JNIEnv, str C.jstring) string {
	if str == 0 {
		return ""
	}
	strlen := C.gio_jni_GetStringLength(env, C.jstring(str))
	chars := C.gio_jni_GetStringChars(env, C.jstring(str))
	var utf16Chars []uint16
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&utf16Chars))
	hdr.Data = uintptr(unsafe.Pointer(chars))
	hdr.Cap = int(strlen)
	hdr.Len = int(strlen)
	utf8 := utf16.Decode(utf16Chars)
	return string(utf8)
}

func Main() {
}

func NewWindow(window Callbacks, opts *Options) error {
	mainWindow.in <- windowAndOptions{window, opts}
	return <-mainWindow.errs
}

func (w *window) WriteClipboard(s string) {
	runOnMain(func(env *C.JNIEnv) {
		jstr := javaString(env, s)
		callStaticVoidMethod(env, android.gioCls, android.mwriteClipboard,
			jvalue(android.appCtx), jvalue(jstr))
	})
}

func (w *window) ReadClipboard() {
	runOnMain(func(env *C.JNIEnv) {
		c, err := callStaticObjectMethod(env, android.gioCls, android.mreadClipboard,
			jvalue(android.appCtx))
		if err != nil {
			return
		}
		content := goString(env, C.jstring(c))
		w.callbacks.Event(system.ClipboardEvent{Text: content})
	})
}

// Close the window. Not implemented for Android.
func (w *window) Close() {}

// RunOnMain is the exported version of runOnMain without a JNI
// environement.
func RunOnMain(f func()) {
	runOnMain(func(_ *C.JNIEnv) {
		f()
	})
}

// runOnMain runs a function on the Java main thread.
func runOnMain(f func(env *C.JNIEnv)) {
	go func() {
		mainFuncs <- f
		runInJVM(javaVM(), func(env *C.JNIEnv) {
			callStaticVoidMethod(env, android.gioCls, android.mwakeupMainThread)
		})
	}()
}

//export Java_org_gioui_Gio_scheduleMainFuncs
func Java_org_gioui_Gio_scheduleMainFuncs(env *C.JNIEnv, cls C.jclass) {
	for {
		select {
		case f := <-mainFuncs:
			f(env)
		default:
			return
		}
	}
}
