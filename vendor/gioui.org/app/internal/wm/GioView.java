// SPDX-License-Identifier: Unlicense OR MIT

package org.gioui;

import java.lang.Class;
import java.lang.IllegalAccessException;
import java.lang.InstantiationException;
import java.lang.ExceptionInInitializerError;
import java.lang.SecurityException;
import android.app.Activity;
import android.app.Fragment;
import android.app.FragmentManager;
import android.app.FragmentTransaction;
import android.content.Context;
import android.graphics.Color;
import android.graphics.Rect;
import android.os.Build;
import android.text.Editable;
import android.util.AttributeSet;
import android.util.TypedValue;
import android.view.Choreographer;
import android.view.KeyCharacterMap;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.PointerIcon;
import android.view.View;
import android.view.ViewConfiguration;
import android.view.WindowInsets;
import android.view.Surface;
import android.view.SurfaceView;
import android.view.SurfaceHolder;
import android.view.inputmethod.BaseInputConnection;
import android.view.inputmethod.InputConnection;
import android.view.inputmethod.InputMethodManager;
import android.view.inputmethod.EditorInfo;

import java.io.UnsupportedEncodingException;

public final class GioView extends SurfaceView implements Choreographer.FrameCallback {
	private static boolean jniLoaded;

	private final SurfaceHolder.Callback surfCallbacks;
	private final View.OnFocusChangeListener focusCallback;
	private final InputMethodManager imm;
	private final float scrollXScale;
	private final float scrollYScale;

	private long nhandle;

	public GioView(Context context) {
		this(context, null);
	}

	public GioView(Context context, AttributeSet attrs) {
		super(context, attrs);

		// Late initialization of the Go runtime to wait for a valid context.
		Gio.init(context.getApplicationContext());

		// Set background color to transparent to avoid a flickering
		// issue on ChromeOS.
		setBackgroundColor(Color.argb(0, 0, 0, 0));

		ViewConfiguration conf = ViewConfiguration.get(context);
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
			scrollXScale = conf.getScaledHorizontalScrollFactor();
			scrollYScale = conf.getScaledVerticalScrollFactor();

			// The platform focus highlight is not aware of Gio's widgets.
			setDefaultFocusHighlightEnabled(false);
		} else {
			float listItemHeight = 48; // dp
			float px = TypedValue.applyDimension(
				TypedValue.COMPLEX_UNIT_DIP,
				listItemHeight,
				getResources().getDisplayMetrics()
			);
			scrollXScale = px;
			scrollYScale = px;
		}

		nhandle = onCreateView(this);
		imm = (InputMethodManager)context.getSystemService(Context.INPUT_METHOD_SERVICE);
		setFocusable(true);
		setFocusableInTouchMode(true);
		focusCallback = new View.OnFocusChangeListener() {
			@Override public void onFocusChange(View v, boolean focus) {
				GioView.this.onFocusChange(nhandle, focus);
			}
		};
		setOnFocusChangeListener(focusCallback);
		surfCallbacks = new SurfaceHolder.Callback() {
			@Override public void surfaceCreated(SurfaceHolder holder) {
				// Ignore; surfaceChanged is guaranteed to be called immediately after this.
			}
			@Override public void surfaceChanged(SurfaceHolder holder, int format, int width, int height) {
				onSurfaceChanged(nhandle, getHolder().getSurface());
			}
			@Override public void surfaceDestroyed(SurfaceHolder holder) {
				onSurfaceDestroyed(nhandle);
			}
		};
		getHolder().addCallback(surfCallbacks);
	}

	@Override public boolean onKeyDown(int keyCode, KeyEvent event) {
		onKeyEvent(nhandle, keyCode, event.getUnicodeChar(), event.getEventTime());
		return false;
	}

	@Override public boolean onGenericMotionEvent(MotionEvent event) {
		dispatchMotionEvent(event);
		return true;
	}

	@Override public boolean onTouchEvent(MotionEvent event) {
		// Ask for unbuffered events. Flutter and Chrome do it
		// so assume it's good for us as well.
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
			requestUnbufferedDispatch(event);
		}

		dispatchMotionEvent(event);
		return true;
	}

	private void setCursor(int id) {
		if (Build.VERSION.SDK_INT < Build.VERSION_CODES.N) {
			return;
		}
		PointerIcon pointerIcon = PointerIcon.getSystemIcon(getContext(), id);
		setPointerIcon(pointerIcon);
	}

	private void dispatchMotionEvent(MotionEvent event) {
		for (int j = 0; j < event.getHistorySize(); j++) {
			long time = event.getHistoricalEventTime(j);
			for (int i = 0; i < event.getPointerCount(); i++) {
				onTouchEvent(
						nhandle,
						event.ACTION_MOVE,
						event.getPointerId(i),
						event.getToolType(i),
						event.getHistoricalX(i, j),
						event.getHistoricalY(i, j),
						scrollXScale*event.getHistoricalAxisValue(MotionEvent.AXIS_HSCROLL, i, j),
						scrollYScale*event.getHistoricalAxisValue(MotionEvent.AXIS_VSCROLL, i, j),
						event.getButtonState(),
						time);
			}
		}
		int act = event.getActionMasked();
		int idx = event.getActionIndex();
		for (int i = 0; i < event.getPointerCount(); i++) {
			int pact = event.ACTION_MOVE;
			if (i == idx) {
				pact = act;
			}
			onTouchEvent(
					nhandle,
					pact,
					event.getPointerId(i),
					event.getToolType(i),
					event.getX(i), event.getY(i),
					scrollXScale*event.getAxisValue(MotionEvent.AXIS_HSCROLL, i),
					scrollYScale*event.getAxisValue(MotionEvent.AXIS_VSCROLL, i),
					event.getButtonState(),
					event.getEventTime());
		}
	}

	@Override public InputConnection onCreateInputConnection(EditorInfo outAttrs) {
		return new InputConnection(this);
	}

	void showTextInput() {
		GioView.this.requestFocus();
		imm.showSoftInput(GioView.this, 0);
	}

	void hideTextInput() {
		imm.hideSoftInputFromWindow(getWindowToken(), 0);
	}

	@Override protected boolean fitSystemWindows(Rect insets) {
		onWindowInsets(nhandle, insets.top, insets.right, insets.bottom, insets.left);
		return true;
	}

	void postFrameCallback() {
		Choreographer.getInstance().removeFrameCallback(this);
		Choreographer.getInstance().postFrameCallback(this);
	}

	@Override public void doFrame(long nanos) {
		onFrameCallback(nhandle, nanos);
	}

	int getDensity() {
		return getResources().getDisplayMetrics().densityDpi;
	}

	float getFontScale() {
		return getResources().getConfiguration().fontScale;
	}

	void start() {
		onStartView(nhandle);
	}

	void stop() {
		onStopView(nhandle);
	}

	void destroy() {
		setOnFocusChangeListener(null);
		getHolder().removeCallback(surfCallbacks);
		onDestroyView(nhandle);
		nhandle = 0;
	}

	void configurationChanged() {
		onConfigurationChanged(nhandle);
	}

	void lowMemory() {
		onLowMemory();
	}

	boolean backPressed() {
		return onBack(nhandle);
	}

	static private native long onCreateView(GioView view);
	static private native void onDestroyView(long handle);
	static private native void onStartView(long handle);
	static private native void onStopView(long handle);
	static private native void onSurfaceDestroyed(long handle);
	static private native void onSurfaceChanged(long handle, Surface surface);
	static private native void onConfigurationChanged(long handle);
	static private native void onWindowInsets(long handle, int top, int right, int bottom, int left);
	static private native void onLowMemory();
	static private native void onTouchEvent(long handle, int action, int pointerID, int tool, float x, float y, float scrollX, float scrollY, int buttons, long time);
	static private native void onKeyEvent(long handle, int code, int character, long time);
	static private native void onFrameCallback(long handle, long nanos);
	static private native boolean onBack(long handle);
	static private native void onFocusChange(long handle, boolean focus);

	private static class InputConnection extends BaseInputConnection {
		private final Editable editable;

		InputConnection(View view) {
			// Passing false enables "dummy mode", where the BaseInputConnection
			// attempts to convert IME operations to key events.
			super(view, false);
			editable = Editable.Factory.getInstance().newEditable("");
		}

		@Override public Editable getEditable() {
			return editable;
		}
	}
}
