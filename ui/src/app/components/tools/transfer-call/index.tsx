import { FC } from 'react';
import { TextInput } from '@/app/components/carbon/form';
import {
  ConfigureToolProps,
  ToolDefinitionForm,
  useParameterManager,
} from '../common';

export const ConfigureTransferCall: FC<ConfigureToolProps> = ({
  toolDefinition,
  onChangeToolDefinition,
  inputClass,
  parameters,
  onParameterChange,
}) => {
  const { getParamValue, updateParameter } = useParameterManager(
    parameters,
    onParameterChange,
  );

  return (
    <>
      <div className="px-6 pb-6">
        <div className="flex flex-col gap-6 max-w-6xl">
          <TextInput
            id="transfer-to"
            labelText="Transfer to"
            helperText="The phone number or SIP URI to transfer calls to (e.g. +14155551234 or sip:agent@example.com)."
            value={getParamValue('tool.transfer_to')}
            onChange={e => updateParameter('tool.transfer_to', e.target.value)}
            placeholder="+14155551234"
          />
        </div>
      </div>

      {toolDefinition && onChangeToolDefinition && (
        <ToolDefinitionForm
          toolDefinition={toolDefinition}
          onChangeToolDefinition={onChangeToolDefinition}
          inputClass={inputClass}
          documentationUrl="https://doc.rapida.ai/assistants/tools/add-transfer-call-tool"
          documentationTitle="Know more about Transfer Call that can be supported by rapida"
        />
      )}
    </>
  );
};
