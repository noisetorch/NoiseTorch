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
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Rect;
import android.os.Build;
import android.os.Bundle;
import android.text.Editable;
import android.util.AttributeSet;
import android.util.TypedValue;
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
import android.view.Window;
import android.view.WindowInsetsController;
import android.view.WindowManager;
import android.view.inputmethod.BaseInputConnection;
import android.view.inputmethod.InputConnection;
import android.view.inputmethod.InputMethodManager;
import android.view.inputmethod.EditorInfo;
import android.text.InputType;
import android.view.accessibility.AccessibilityNodeProvider;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.accessibility.AccessibilityEvent;
import android.view.accessibility.AccessibilityManager;

import java.io.UnsupportedEncodingException;

public final class GioView extends SurfaceView {
	private static boolean jniLoaded;

	private final SurfaceHolder.Callback surfCallbacks;
	private final View.OnFocusChangeListener focusCallback;
	private final InputMethodManager imm;
	private final float scrollXScale;
	private final float scrollYScale;
	private int keyboardHint;
	private AccessibilityManager accessManager;

	private long nhandle;

	public GioView(Context context) {
		this(context, null);
	}

	public GioView(Context context, AttributeSet attrs) {
		super(context, attrs);
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
			setSystemUiVisibility(View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION | View.SYSTEM_UI_FLAG_LAYOUT_STABLE);
		}
		setLayoutParams(new WindowManager.LayoutParams(WindowManager.LayoutParams.MATCH_PARENT, WindowManager.LayoutParams.MATCH_PARENT));

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

		accessManager = (AccessibilityManager)context.getSystemService(Context.ACCESSIBILITY_SERVICE);
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
		if (nhandle != 0) {
			onKeyEvent(nhandle, keyCode, event.getUnicodeChar(), event.getEventTime());
		}
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

	private void setOrientation(int id, int fallback) {
		if (Build.VERSION.SDK_INT < Build.VERSION_CODES.JELLY_BEAN_MR2) {
			id = fallback;
		}
		((Activity) this.getContext()).setRequestedOrientation(id);
	}

	private void setFullscreen(boolean enabled) {
		int flags = this.getSystemUiVisibility();
		if (enabled) {
		   flags |= SYSTEM_UI_FLAG_IMMERSIVE_STICKY;
		   flags |= SYSTEM_UI_FLAG_HIDE_NAVIGATION;
		   flags |= SYSTEM_UI_FLAG_FULLSCREEN;
		   flags |= SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN;
		} else {
		   flags &= ~SYSTEM_UI_FLAG_IMMERSIVE_STICKY;
		   flags &= ~SYSTEM_UI_FLAG_HIDE_NAVIGATION;
		   flags &= ~SYSTEM_UI_FLAG_FULLSCREEN;
		   flags &= ~SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN;
		}
		this.setSystemUiVisibility(flags);
	}

	private enum Bar {
		NAVIGATION,
		STATUS,
	}

	private void setBarColor(Bar t, int color, int luminance) {
		if (Build.VERSION.SDK_INT < Build.VERSION_CODES.LOLLIPOP) {
			return;
		}

		Window window = ((Activity) this.getContext()).getWindow();

		int insetsMask;
		int viewMask;

		switch (t) {
		case STATUS:
			insetsMask = WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS;
			viewMask = View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
			window.setStatusBarColor(color);
			break;
		case NAVIGATION:
			insetsMask = WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS;
			viewMask = View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
			window.setNavigationBarColor(color);
			break;
		default:
			throw new RuntimeException("invalid bar type");
		}

		if (Build.VERSION.SDK_INT < Build.VERSION_CODES.M) {
			return;
		}

		if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) {
			int flags = this.getSystemUiVisibility();
			if (luminance > 128) {
			   flags |=  viewMask;
			} else {
			   flags &= ~viewMask;
			}
			this.setSystemUiVisibility(flags);
			return;
		}

		WindowInsetsController insetsController = window.getInsetsController();
		if (insetsController == null) {
			return;
		}
		if (luminance > 128) {
			insetsController.setSystemBarsAppearance(insetsMask, insetsMask);
		} else {
			insetsController.setSystemBarsAppearance(0, insetsMask);
		}
	}

	private void setStatusColor(int color, int luminance) {
		this.setBarColor(Bar.STATUS, color, luminance);
	}

	private void setNavigationColor(int color, int luminance) {
		this.setBarColor(Bar.NAVIGATION, color, luminance);
	}

	@Override protected boolean dispatchHoverEvent(MotionEvent event) {
		if (!accessManager.isTouchExplorationEnabled()) {
			return super.dispatchHoverEvent(event);
		}
		switch (event.getAction()) {
		case MotionEvent.ACTION_HOVER_ENTER:
			// Fall through.
		case MotionEvent.ACTION_HOVER_MOVE:
			onTouchExploration(nhandle, event.getX(), event.getY());
			break;
		case MotionEvent.ACTION_HOVER_EXIT:
			onExitTouchExploration(nhandle);
			break;
		}
		return true;
	}

	void sendA11yEvent(int eventType, int viewId) {
		if (!accessManager.isEnabled()) {
			return;
		}
		AccessibilityEvent event = obtainA11yEvent(eventType, viewId);
		getParent().requestSendAccessibilityEvent(this, event);
	}

	AccessibilityEvent obtainA11yEvent(int eventType, int viewId) {
		AccessibilityEvent event = AccessibilityEvent.obtain(eventType);
		event.setPackageName(getContext().getPackageName());
		event.setSource(this, viewId);
		return event;
	}

	boolean isA11yActive() {
		return accessManager.isEnabled();
	}

	void sendA11yChange(int viewId) {
		if (!accessManager.isEnabled()) {
			return;
		}
		AccessibilityEvent event = obtainA11yEvent(AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED, viewId);
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT) {
			event.setContentChangeTypes(AccessibilityEvent.CONTENT_CHANGE_TYPE_SUBTREE);
		}
		getParent().requestSendAccessibilityEvent(this, event);
	}

	private void dispatchMotionEvent(MotionEvent event) {
		if (nhandle == 0) {
			return;
		}
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

	@Override public InputConnection onCreateInputConnection(EditorInfo editor) {
		editor.inputType = this.keyboardHint;
		editor.imeOptions = EditorInfo.IME_FLAG_NO_FULLSCREEN | EditorInfo.IME_FLAG_NO_EXTRACT_UI;
		return new InputConnection(this);
	}

	void setInputHint(int hint) {
		if (hint == this.keyboardHint) {
			return;
		}
		this.keyboardHint = hint;
		imm.restartInput(this);
	}

	void showTextInput() {
		GioView.this.requestFocus();
		imm.showSoftInput(GioView.this, 0);
	}

	void hideTextInput() {
		imm.hideSoftInputFromWindow(getWindowToken(), 0);
	}

	@Override protected boolean fitSystemWindows(Rect insets) {
		if (nhandle != 0) {
			onWindowInsets(nhandle, insets.top, insets.right, insets.bottom, insets.left);
		}
		return true;
	}

	@Override protected void onDraw(Canvas canvas) {
		if (nhandle != 0) {
			onFrameCallback(nhandle);
		}
	}

	int getDensity() {
		return getResources().getDisplayMetrics().densityDpi;
	}

	float getFontScale() {
		return getResources().getConfiguration().fontScale;
	}

	public void start() {
		if (nhandle != 0) {
			onStartView(nhandle);
		}
	}

	public void stop() {
		if (nhandle != 0) {
			onStopView(nhandle);
		}
	}

	public void destroy() {
		if (nhandle != 0) {
			onDestroyView(nhandle);
		}
	}

	protected void unregister() {
		setOnFocusChangeListener(null);
		getHolder().removeCallback(surfCallbacks);
		nhandle = 0;
	}

	public void configurationChanged() {
		if (nhandle != 0) {
			onConfigurationChanged(nhandle);
		}
	}

	public boolean backPressed() {
		if (nhandle == 0) {
			return false;
		}
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
	static public native void onLowMemory();
	static private native void onTouchEvent(long handle, int action, int pointerID, int tool, float x, float y, float scrollX, float scrollY, int buttons, long time);
	static private native void onKeyEvent(long handle, int code, int character, long time);
	static private native void onFrameCallback(long handle);
	static private native boolean onBack(long handle);
	static private native void onFocusChange(long handle, boolean focus);
	static private native AccessibilityNodeInfo initializeAccessibilityNodeInfo(long handle, int viewId, int screenX, int screenY, AccessibilityNodeInfo info);
	static private native void onTouchExploration(long handle, float x, float y);
	static private native void onExitTouchExploration(long handle);
	static private native void onA11yFocus(long handle, int viewId);
	static private native void onClearA11yFocus(long handle, int viewId);

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

	@Override public AccessibilityNodeProvider getAccessibilityNodeProvider() {
		return new AccessibilityNodeProvider() {
			private final int[] screenOff = new int[2];

			@Override public AccessibilityNodeInfo createAccessibilityNodeInfo(int viewId) {
				AccessibilityNodeInfo info = null;
				if (viewId == View.NO_ID) {
					info = AccessibilityNodeInfo.obtain(GioView.this);
					GioView.this.onInitializeAccessibilityNodeInfo(info);
				} else {
					info = AccessibilityNodeInfo.obtain(GioView.this, viewId);
					info.setPackageName(getContext().getPackageName());
					info.setVisibleToUser(true);
				}
				GioView.this.getLocationOnScreen(screenOff);
				info = GioView.this.initializeAccessibilityNodeInfo(nhandle, viewId, screenOff[0], screenOff[1], info);
				return info;
			}

			@Override public boolean performAction(int viewId, int action, Bundle arguments) {
				if (viewId == View.NO_ID) {
					return GioView.this.performAccessibilityAction(action, arguments);
				}
				switch (action) {
				case AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS:
					GioView.this.onA11yFocus(nhandle, viewId);
					GioView.this.sendA11yEvent(AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUSED, viewId);
					return true;
				case AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS:
					GioView.this.onClearA11yFocus(nhandle, viewId);
					GioView.this.sendA11yEvent(AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUS_CLEARED, viewId);
					return true;
				}
				return false;
			}
		};
	}
}
