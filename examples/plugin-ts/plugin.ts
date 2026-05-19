// Plugin entrypoint. Edit this file as needed.
import { serve } from 'pluginart';
import { CONTRACT_HASH } from './plugin/contract';
import { handle } from './handler';

serve(handle, { contractHash: CONTRACT_HASH });
