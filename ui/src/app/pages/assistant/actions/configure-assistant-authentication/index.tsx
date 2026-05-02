import { FC, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import {
  Breadcrumb,
  BreadcrumbItem,
  ButtonSet,
  CheckboxGroup,
  Slider,
  Select as CarbonSelect,
  SelectItem,
} from '@carbon/react';
import { TextInput, Stack } from '@/app/components/carbon/form';
import { InputCheckbox } from '@/app/components/carbon/form/input-checkbox';
import { PrimaryButton, SecondaryButton } from '@/app/components/carbon/button';
import { useGlobalNavigation } from '@/hooks/use-global-navigator';
import { useConfirmDialog } from '@/app/pages/assistant/actions/hooks/use-confirmation';
import { InputGroup } from '@/app/components/input-group';
import { APiStringHeader } from '@/app/components/external-api/api-header';
import {
  ASSISTANT_CONDITION_KEY_OPTIONS,
  ASSISTANT_CONDITION_OPERATOR_OPTIONS,
  ASSISTANT_CONDITION_SOURCE_OPTIONS,
  ASSISTANT_CONDITION_VALUE_OPTIONS_BY_KEY,
  ParameterEditor,
} from '@/app/components/tools/common';
import { SourceConditionRule } from '@/app/components/conditions/source-condition-rule';

type AuthProvider = 'api';
type HttpMethod = 'POST' | 'GET';
type FailBehavior = 'block' | 'none';
const AUTH_PARAMETER_TYPE_OPTIONS = [
  { value: 'client', name: 'Client' },
  { value: 'assistant', name: 'Assistant' },
  { value: 'conversation', name: 'Conversation' },
  { value: 'argument', name: 'Argument' },
  { value: 'metadata', name: 'Metadata' },
  { value: 'option', name: 'Option' },
  { value: 'custom', name: 'Custom' },
];

export function ConfigureAssistantAuthenticationPage() {
  const { assistantId } = useParams();
  return (
    <>
      {assistantId && (
        <ConfigureAssistantAuthentication assistantId={assistantId} />
      )}
    </>
  );
}

const ConfigureAssistantAuthentication: FC<{ assistantId: string }> = ({
  assistantId,
}) => {
  const navigator = useGlobalNavigation();
  const { showDialog, ConfirmDialogComponent } = useConfirmDialog({});

  const [enabled, setEnabled] = useState(false);
  const [provider, setProvider] = useState<AuthProvider>('api');
  const [endpoint, setEndpoint] = useState('');
  const [method, setMethod] = useState<HttpMethod>('POST');
  const [timeout, setTimeoutValue] = useState(5000);
  const [failBehavior, setFailBehavior] = useState<FailBehavior>('block');
  const [headers, setHeaders] = useState('{}');
  const [body, setBody] = useState(
    '{"assistant.id":"assistantId","client.phone":"clientPhone"}',
  );
  const [sourceConditions, setSourceConditions] = useState([
    {
      key: 'source',
      condition: '=',
      value: 'all',
    },
  ]);

  const canSave = useMemo(() => {
    if (!enabled) return true;
    if (!endpoint.trim()) return false;
    try {
      JSON.parse(headers || '{}');
      JSON.parse(body || '{}');
      return true;
    } catch {
      return false;
    }
  }, [enabled, endpoint, headers, body]);

  return (
    <>
      <ConfirmDialogComponent />
      <div className="flex flex-col flex-1 min-h-0 bg-white dark:bg-gray-900">
        <div className="px-4 pt-4 pb-6 border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900">
          <div>
            <Breadcrumb noTrailingSlash className="mb-2">
              <BreadcrumbItem
                href={`/deployment/assistant/${assistantId}/overview`}
              >
                Assistant
              </BreadcrumbItem>
            </Breadcrumb>
            <h1 className="text-2xl font-light tracking-tight">
              Authentication
            </h1>
          </div>
        </div>

        <div className="flex-1 min-h-0 overflow-auto">
          <div className="p-6">
            <CheckboxGroup
              legendText=""
              warn
              warnText={
                enabled
                  ? 'All sessions must be verified before initialization.'
                  : 'Authentication is disabled. Sessions will continue without verification.'
              }
            >
              <InputCheckbox
                id="assistant-auth-enabled"
                checked={enabled}
                onChange={e => setEnabled(e.target.checked)}
              >
                Enable Session Authentication
              </InputCheckbox>
            </CheckboxGroup>
          </div>

          {enabled && (
            <>
              <InputGroup title="Condition">
                <SourceConditionRule
                  conditions={sourceConditions}
                  onChangeConditions={setSourceConditions}
                  conditionOptions={ASSISTANT_CONDITION_OPERATOR_OPTIONS}
                  sourceOptions={ASSISTANT_CONDITION_SOURCE_OPTIONS}
                  keyOptions={ASSISTANT_CONDITION_KEY_OPTIONS}
                  valueOptionsByKey={ASSISTANT_CONDITION_VALUE_OPTIONS_BY_KEY}
                  keyTooltipText="The variable to evaluate for this condition."
                />
              </InputGroup>
              <InputGroup title="Provider">
                <CarbonSelect
                  id="assistant-auth-provider"
                  labelText="Authentication Provider"
                  value={provider}
                  onChange={e => setProvider(e.target.value as AuthProvider)}
                  disabled={!enabled}
                >
                  <SelectItem value="api" text="API" />
                </CarbonSelect>
              </InputGroup>
              <InputGroup title="Definition">
                <Stack gap={7}>
                  <div className="flex space-x-2">
                    <div className="relative w-40">
                      <CarbonSelect
                        id="assistant-auth-method"
                        labelText="Method"
                        value={method}
                        onChange={e => setMethod(e.target.value as HttpMethod)}
                        disabled={provider !== 'api'}
                      >
                        <SelectItem value="POST" text="POST" />
                        <SelectItem value="GET" text="GET" />
                      </CarbonSelect>
                    </div>
                    <div className="relative w-full">
                      <TextInput
                        id="assistant-auth-endpoint"
                        labelText="Server URL"
                        value={endpoint}
                        onChange={e => setEndpoint(e.target.value)}
                        placeholder="https://auth.example.com/resolve"
                        disabled={provider !== 'api'}
                      />
                    </div>
                  </div>

                  <div className="flex space-x-2">
                    <div className="relative w-40">
                      <CarbonSelect
                        id="assistant-auth-fail-behavior"
                        labelText="On Error"
                        value={failBehavior}
                        onChange={e =>
                          setFailBehavior(e.target.value as FailBehavior)
                        }
                        disabled={provider !== 'api'}
                      >
                        <SelectItem value="block" text="Block" />
                        <SelectItem value="none" text="Do nothing" />
                      </CarbonSelect>
                    </div>
                    <div className="relative w-full">
                      <Slider
                        id="assistant-auth-timeout"
                        labelText="Timeout (ms)"
                        value={timeout}
                        min={500}
                        max={10000}
                        step={100}
                        onChange={(data: { value: number | number[] }) =>
                          setTimeoutValue(
                            Array.isArray(data.value)
                              ? data.value[0]
                              : data.value,
                          )
                        }
                        disabled={provider !== 'api'}
                      />
                    </div>
                  </div>

                  <div>
                    <p className="text-xs font-medium mb-2">Headers</p>
                    <APiStringHeader
                      headerValue={headers}
                      setHeaderValue={setHeaders}
                    />
                  </div>

                  <ParameterEditor
                    value={body}
                    onChange={setBody}
                    typeOptions={AUTH_PARAMETER_TYPE_OPTIONS}
                    includeEmptyKeyOption
                  />
                </Stack>
              </InputGroup>
            </>
          )}
        </div>

        <ButtonSet className="!w-full [&>button]:!flex-1 [&>button]:!max-w-none">
          <SecondaryButton
            size="lg"
            onClick={() => showDialog(navigator.goBack)}
          >
            Cancel
          </SecondaryButton>
          <PrimaryButton size="lg" disabled={!canSave}>
            Save authentication
          </PrimaryButton>
        </ButtonSet>
      </div>
    </>
  );
};
