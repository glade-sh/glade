import { event, lifecycle } from '@fds-uif/behavior';

export const handleClick = event<MouseEvent>({
  description: 'Click fired when button is activated',
  handler: (e, { props, state, refs }) => {
    // Prevent dispatch if disabled
    if (props.disabled) {
      e.preventDefault();
      return false;
    }

    // refs.host references host.template in lwc
    const form = refs.host.closest('form');
    if (form) {
        form.requestSubmit();
    }

    // Call user's handler
    props.onClick?.(e);
  }
});

export const handleButtonFocus = event<FocusEvent>({
  description: 'Focus fired when button is activated',
  handler: (e, { props, state }) => {
    props.onFocus?.(e);
  }
});

export const handleButtonBlur = event<FocusEvent>({
  description: 'Blur fired when button is activated',
  handler: (e, { props, state }) => {
    props.onBlur?.(e);
  }
});

// Lifecycle
export const mount = lifecycle.mount(({ props, refs }) => {
  refs.host.style.pointerEvents = props.disabled ? 'none' : '';
});

