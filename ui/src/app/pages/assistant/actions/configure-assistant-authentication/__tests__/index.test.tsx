import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import { ConfigureAssistantAuthenticationPage } from '../index';

jest.mock('react-router-dom', () => ({
  ...jest.requireActual('react-router-dom'),
  useParams: () => ({ assistantId: 'assistant-1' }),
}));

jest.mock('@/hooks/use-global-navigator', () => ({
  useGlobalNavigation: () => ({
    goBack: jest.fn(),
  }),
}));

jest.mock('@/app/pages/assistant/actions/hooks/use-confirmation', () => ({
  useConfirmDialog: () => ({
    showDialog: (cb: () => void) => cb(),
    ConfirmDialogComponent: () => null,
  }),
}));

jest.mock('@/app/components/input-group', () => ({
  InputGroup: ({ title, children }: any) => (
    <section>
      {title ? <div>{title}</div> : null}
      {children}
    </section>
  ),
}));

jest.mock('@/app/components/conditions/source-condition-rule', () => ({
  SourceConditionRule: () => <div>conditions</div>,
}));

jest.mock('@/app/components/external-api/api-header', () => ({
  APiStringHeader: () => <div>headers</div>,
}));

jest.mock('@/app/components/carbon/form/input-checkbox', () => ({
  InputCheckbox: ({ id, checked, onChange, children }: any) => (
    <label htmlFor={id}>
      <input id={id} type="checkbox" checked={checked} onChange={onChange} />
      {children}
    </label>
  ),
}));

jest.mock('@/app/components/carbon/form', () => ({
  Stack: ({ children }: any) => <div>{children}</div>,
  TextInput: ({ id, labelText, value, onChange, hideLabel }: any) => (
    <div>
      {!hideLabel && labelText ? <label htmlFor={id}>{labelText}</label> : null}
      <input id={id} data-testid={id} value={value ?? ''} onChange={onChange} />
    </div>
  ),
  TextArea: ({ id, value, onChange }: any) => (
    <textarea id={id} value={value ?? ''} onChange={onChange} />
  ),
}));

jest.mock('@/app/components/carbon/button', () => ({
  PrimaryButton: ({ children, ...props }: any) => (
    <button {...props}>{children}</button>
  ),
  SecondaryButton: ({ children, ...props }: any) => (
    <button {...props}>{children}</button>
  ),
  TertiaryButton: ({ children, ...props }: any) => (
    <button {...props}>{children}</button>
  ),
}));

jest.mock('@carbon/react', () => ({
  Breadcrumb: ({ children }: any) => <div>{children}</div>,
  BreadcrumbItem: ({ children }: any) => <span>{children}</span>,
  ButtonSet: ({ children }: any) => <div>{children}</div>,
  CheckboxGroup: ({ children }: any) => <div>{children}</div>,
  Slider: ({ id, value, onChange }: any) => (
    <input
      id={id}
      type="range"
      value={value}
      onChange={e => onChange?.({ value: Number(e.target.value) })}
    />
  ),
  Select: ({ id, labelText, value, onChange, children, hideLabel }: any) => (
    <div>
      {!hideLabel && labelText ? <label htmlFor={id}>{labelText}</label> : null}
      <select id={id} data-testid={id} value={value} onChange={onChange}>
        {children}
      </select>
    </div>
  ),
  SelectItem: ({ value, text }: any) => <option value={value}>{text}</option>,
  Button: ({ children, iconDescription, ...props }: any) => (
    <button aria-label={iconDescription || children || 'button'} {...props}>
      {children || 'button'}
    </button>
  ),
  Tooltip: ({ children }: any) => <span>{children}</span>,
}));

describe('ConfigureAssistantAuthenticationPage', () => {
  it('supports add and edit for authentication parameter mapping', () => {
    render(<ConfigureAssistantAuthenticationPage />);

    fireEvent.click(screen.getByLabelText('Enable Session Authentication'));
    fireEvent.change(screen.getByTestId('assistant-auth-endpoint'), {
      target: { value: 'https://auth.example.com/resolve' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Add parameter' }));
    fireEvent.change(screen.getByTestId('param-val-2'), {
      target: { value: 'assistantPrompt' },
    });

    expect(screen.getByText('Mapping (3)')).toBeInTheDocument();
    expect(screen.getByTestId('param-val-2')).toHaveValue('assistantPrompt');
  });
});
